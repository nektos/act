package runner

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

// errSupersededByConcurrencyGroup is returned by acquire when the concurrency
// group itself cancelled the request: a newer request superseded it or the
// queue of the group was full.
var errSupersededByConcurrencyGroup = errors.New("superseded by a newer request in the concurrency group")

// errConcurrencyWaitCancelled is returned by acquire when the requester was
// cancelled while waiting for the group, for example by Ctrl+C or by the
// cancellation of its own workflow run. The group did not cancel it.
var errConcurrencyWaitCancelled = errors.New("cancelled while waiting for the concurrency group")

// isConcurrencyCancellation reports whether acquire declined the request
// rather than failing, in which case the job or run is marked 'cancelled'
func isConcurrencyCancellation(err error) bool {
	return errors.Is(err, errSupersededByConcurrencyGroup) || errors.Is(err, errConcurrencyWaitCancelled)
}

// maxPendingRuns is the number of jobs or workflow runs that may be pending
// in one concurrency group with `queue: max`
const maxPendingRuns = 100

// concurrencySpec is an evaluated `concurrency` configuration
type concurrencySpec struct {
	// group is the group name as written, used for log messages. Groups are
	// identified by concurrencyGroupKey, never by this field.
	group            string
	cancelInProgress bool
	// queueMax allows up to maxPendingRuns pending requests instead of the
	// default single pending request (`queue: max`)
	queueMax bool
}

// concurrencyGroupKey identifies a concurrency group. GitHub treats group
// names case insensitively, so `prod` and `Prod` are the same group.
func concurrencyGroupKey(group string) string {
	return strings.ToLower(group)
}

func queuePolicyName(queueMax bool) string {
	if queueMax {
		return "max"
	}
	return "single"
}

// concurrencyWaiter represents one workflow run or job holding or waiting
// for a concurrency group
type concurrencyWaiter struct {
	// cancelSelf gracefully cancels the holder when a new request arrives
	// with cancel-in-progress
	cancelSelf func()
	// promoted is closed when the waiter becomes the holder of the group
	promoted chan struct{}
	// superseded is closed when a pending request is cancelled without
	// having run, because a newer request replaced it
	superseded chan struct{}
}

type concurrencyGroup struct {
	holder *concurrencyWaiter
	// pending holds the waiting requests in first-in-first-out order, the
	// head is promoted when the holder releases the group
	pending []*concurrencyWaiter
	// queueMax is the queue policy of the group, taken from the request that
	// created it. Workflows sharing a group name are expected to agree on
	// the policy; the group's policy wins for later arrivals so that a
	// workflow which omits `queue: max` cannot discard a queue admitted
	// under it.
	queueMax bool
}

// concurrencyManager implements GitHub's concurrency group queueing: one
// holder runs at a time and requests queue behind it. With the default
// `queue: single` at most one request is pending and a newer request
// supersedes (cancels) the previously pending one; with `queue: max` up to
// maxPendingRuns requests queue in FIFO order and requests arriving once the
// queue is full are cancelled. A single instance is shared between a runner
// and all reusable workflow runners spawned from it.
type concurrencyManager struct {
	mu     sync.Mutex
	groups map[string]*concurrencyGroup
}

func newConcurrencyManager() *concurrencyManager {
	return &concurrencyManager{groups: map[string]*concurrencyGroup{}}
}

// acquire blocks until the group is free and returns a function releasing it
// again. cancelSelf is invoked if a later request cancels this one via
// cancel-in-progress while it holds the group. cancelCtx (optional) aborts
// waiting when the requester itself is cancelled.
func (m *concurrencyManager) acquire(ctx context.Context, cancelCtx context.Context, spec concurrencySpec, cancelSelf func()) (func(), error) {
	logger := common.Logger(ctx)
	me := &concurrencyWaiter{
		cancelSelf: cancelSelf,
		promoted:   make(chan struct{}),
		superseded: make(chan struct{}),
	}

	key := concurrencyGroupKey(spec.group)

	m.mu.Lock()
	group := m.groups[key]
	if group == nil {
		group = &concurrencyGroup{queueMax: spec.queueMax}
		m.groups[key] = group
	} else if group.queueMax != spec.queueMax {
		// workflows sharing a group name are expected to agree on the queue
		// policy; the policy of the group wins so a workflow that omits
		// `queue: max` cannot discard a queue admitted under it
		logger.Warnf("Concurrency group '%s' is used with both 'queue: single' and 'queue: max', keeping '%s' for the whole group",
			spec.group, queuePolicyName(group.queueMax))
	}
	if group.holder == nil {
		group.holder = me
		m.mu.Unlock()
		return func() { m.release(key, me) }, nil
	}
	if spec.cancelInProgress && group.holder.cancelSelf != nil {
		logger.Infof("Cancelling the run in progress in concurrency group '%s'", spec.group)
		group.holder.cancelSelf()
	}
	if group.queueMax {
		// with `queue: max` the queue is bounded and the *arriving* request
		// is cancelled once it is full, the queued ones keep their place
		if len(group.pending) >= maxPendingRuns {
			m.mu.Unlock()
			logger.Infof("Concurrency group '%s' already has %d pending runs, cancelling this one", spec.group, maxPendingRuns)
			return nil, errSupersededByConcurrencyGroup
		}
	} else if len(group.pending) > 0 {
		// with the default `queue: single` GitHub keeps at most one pending
		// run per group: the previously pending run is cancelled and
		// replaced by the newer request
		if len(group.pending) == 1 {
			logger.Infof("Superseding the run pending in concurrency group '%s'", spec.group)
		} else {
			logger.Infof("Superseding the %d runs pending in concurrency group '%s'", len(group.pending), spec.group)
		}
		for _, superseded := range group.pending {
			close(superseded.superseded)
		}
		group.pending = nil
	}
	group.pending = append(group.pending, me)
	position := len(group.pending)
	queueMax := group.queueMax
	m.mu.Unlock()

	if queueMax {
		logger.Infof("Waiting for concurrency group '%s' at queue position %d", spec.group, position)
	} else {
		logger.Infof("Waiting for concurrency group '%s'", spec.group)
	}

	var cancelDone <-chan struct{}
	if cancelCtx != nil {
		cancelDone = cancelCtx.Done()
	}
	select {
	case <-me.promoted:
		logger.Infof("Acquired concurrency group '%s'", spec.group)
		return func() { m.release(key, me) }, nil
	case <-me.superseded:
		return nil, errSupersededByConcurrencyGroup
	case <-cancelDone:
		m.abandon(key, me)
		return nil, errConcurrencyWaitCancelled
	case <-ctx.Done():
		m.abandon(key, me)
		return nil, ctx.Err()
	}
}

func (m *concurrencyManager) release(groupKey string, me *concurrencyWaiter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	group := m.groups[groupKey]
	if group == nil || group.holder != me {
		return
	}
	m.releaseLocked(groupKey, group)
}

// releaseLocked hands the group to the request that has been waiting the
// longest (FIFO), or drops the group when nothing is waiting
func (m *concurrencyManager) releaseLocked(groupKey string, group *concurrencyGroup) {
	if len(group.pending) > 0 {
		group.holder = group.pending[0]
		group.pending = group.pending[1:]
		close(group.holder.promoted)
		return
	}
	delete(m.groups, groupKey)
}

// abandon drops a waiter that gave up waiting for the group. The waiter can
// be promoted to holder at the very moment it gives up — its select then may
// pick either branch — so a waiter that turns out to be the holder releases
// the group instead of leaving it held by a request that never runs.
func (m *concurrencyManager) abandon(groupKey string, me *concurrencyWaiter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	group := m.groups[groupKey]
	if group == nil {
		return
	}
	for i, waiter := range group.pending {
		if waiter == me {
			group.pending = append(group.pending[:i], group.pending[i+1:]...)
			if group.holder == nil && len(group.pending) == 0 {
				delete(m.groups, groupKey)
			}
			return
		}
	}
	if group.holder == me {
		m.releaseLocked(groupKey, group)
	}
}

// concurrency returns the shared concurrency manager for this config,
// creating it on first use
func (config *Config) concurrency() *concurrencyManager {
	concurrencyInitMu.Lock()
	defer concurrencyInitMu.Unlock()
	if config.concurrencyManager == nil {
		config.concurrencyManager = newConcurrencyManager()
	}
	return config.concurrencyManager
}

var concurrencyInitMu sync.Mutex

// heldConcurrencyGroupsKey carries the set of concurrency groups already held
// by the current workflow run or its callers, so jobs and called reusable
// workflows do not deadlock waiting for a group their own caller holds
type heldConcurrencyGroupsKey struct{}

func withHeldConcurrencyGroup(ctx context.Context, group string) context.Context {
	held := map[string]bool{concurrencyGroupKey(group): true}
	for g := range heldConcurrencyGroups(ctx) {
		held[g] = true
	}
	return context.WithValue(ctx, heldConcurrencyGroupsKey{}, held)
}

func heldConcurrencyGroups(ctx context.Context) map[string]bool {
	if held, ok := ctx.Value(heldConcurrencyGroupsKey{}).(map[string]bool); ok {
		return held
	}
	return nil
}

// isConcurrencyGroupHeld reports whether the group is already held by this
// workflow run or one of its callers, in which case waiting for it would
// deadlock on our own caller
func isConcurrencyGroupHeld(ctx context.Context, group string) bool {
	return heldConcurrencyGroups(ctx)[concurrencyGroupKey(group)]
}

// workflowRunState tracks the cancellation of one workflow run, used both for
// cancel-in-progress of workflow level concurrency groups and to skip jobs of
// a cancelled run that have not started yet
type workflowRunState struct {
	cancelled  atomic.Bool
	completed  atomic.Bool
	cancelCh   chan struct{}
	cancelOnce sync.Once
}

func newWorkflowRunState() *workflowRunState {
	return &workflowRunState{cancelCh: make(chan struct{})}
}

// cancelRun gracefully cancels all jobs of the run, it has no effect once
// the run completed
func (s *workflowRunState) cancelRun() {
	if s.completed.Load() {
		return
	}
	s.cancelled.Store(true)
	s.cancelOnce.Do(func() { close(s.cancelCh) })
}

func (s *workflowRunState) complete() {
	s.completed.Store(true)
}

func evaluateConcurrency(ctx context.Context, ee ExpressionEvaluator, concurrency *model.Concurrency) (concurrencySpec, bool, error) {
	if concurrency == nil {
		return concurrencySpec{}, false, nil
	}
	cancelInProgress := false
	if val := strings.TrimSpace(ee.Interpolate(ctx, concurrency.CancelInProgress)); val != "" {
		var err error
		if cancelInProgress, err = strconv.ParseBool(val); err != nil {
			common.Logger(ctx).Errorf("Failed to parse 'cancel-in-progress' value '%s': %v", val, err)
			cancelInProgress = false
		}
	}
	// the settings are validated before the group is inspected, an invalid
	// concurrency block is rejected even when its group interpolates empty
	queueMax, err := parseQueuePolicy(strings.TrimSpace(ee.Interpolate(ctx, concurrency.Queue)))
	if err != nil {
		return concurrencySpec{}, false, err
	}
	if err := validateQueuePolicy(queueMax, cancelInProgress); err != nil {
		return concurrencySpec{}, false, err
	}

	group := strings.TrimSpace(ee.Interpolate(ctx, concurrency.Group))
	if group == "" {
		common.Logger(ctx).Warnf("Concurrency group evaluated to an empty string, ignoring the concurrency settings")
		return concurrencySpec{}, false, nil
	}
	return concurrencySpec{
		group:            group,
		cancelInProgress: cancelInProgress,
		queueMax:         queueMax,
	}, true, nil
}

// parseQueuePolicy maps the `queue` value to whether the group queues up to
// maxPendingRuns requests. An empty value is the documented default `single`.
func parseQueuePolicy(value string) (bool, error) {
	switch value {
	case "", "single":
		return false, nil
	case "max":
		return true, nil
	default:
		return false, fmt.Errorf("invalid value '%s' for 'concurrency.queue', expected 'single' or 'max'", value)
	}
}

// validateQueuePolicy rejects the combination GitHub refuses to run
func validateQueuePolicy(queueMax bool, cancelInProgress bool) error {
	if queueMax && cancelInProgress {
		return errors.New("the combination of 'queue: max' and 'cancel-in-progress: true' is not allowed in a concurrency group")
	}
	return nil
}

// withConcurrency wraps a job executor so it takes part in the cancellation
// of its workflow run and holds the job level concurrency group of the job
// while running. Jobs cancelled by their group or their run finish with
// result 'cancelled' without failing the plan.
func (rc *RunContext) withConcurrency(executor common.Executor) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		runState := rc.runState

		// jobs of a cancelled run that have not started yet are cancelled
		if runState != nil && runState.cancelled.Load() && ctx.Err() == nil {
			logger.WithField("jobResult", "cancelled").Infof("\U0001F6AB  Job '%s' was cancelled because its workflow run was cancelled", rc.Name)
			rc.cancelledResult()
			return nil
		}

		spec, hasSpec, err := evaluateConcurrency(ctx, rc.NewExpressionEvaluator(ctx), rc.Run.Job().Concurrency())
		if err != nil {
			return err
		}
		if hasSpec && isConcurrencyGroupHeld(ctx, spec.group) {
			// the group is already held by this run or a calling workflow,
			// acquiring it again would deadlock on our own caller
			logger.Debugf("Concurrency group '%s' is already held by this workflow run or its caller", spec.group)
			hasSpec = false
		}

		if !hasSpec && runState == nil {
			return rc.withJobSlot(ctx, executor)
		}

		jobCancelCtx, cancelJob := rc.newJobCancelContext(ctx, runState)
		defer cancelJob()

		var cancelledByGroup atomic.Bool
		var jobCompleted atomic.Bool
		cancelSelf := func() {
			if jobCompleted.Load() {
				return
			}
			cancelledByGroup.Store(true)
			cancelJob()
		}

		if hasSpec {
			release, err := rc.Config.concurrency().acquire(ctx, jobCancelCtx, spec, cancelSelf)
			if err != nil {
				if isConcurrencyCancellation(err) && ctx.Err() == nil {
					if errors.Is(err, errSupersededByConcurrencyGroup) {
						logger.WithField("jobResult", "cancelled").Infof("\U0001F6AB  Job '%s' was cancelled by concurrency group '%s' before it started", rc.Name, spec.group)
					} else {
						logger.WithField("jobResult", "cancelled").Infof("\U0001F6AB  Job '%s' was cancelled while waiting for concurrency group '%s'", rc.Name, spec.group)
					}
					rc.cancelledResult()
					return nil
				}
				return err
			}
			defer release()
			ctx = withHeldConcurrencyGroup(ctx, spec.group)
		}

		err = rc.withJobSlot(ctx, func(ctx context.Context) error {
			return executor(common.WithJobCancelContext(ctx, jobCancelCtx))
		})
		jobCompleted.Store(true)
		if rc.handleCancelledJob(ctx, cancelledByGroup.Load(), spec.group) {
			return nil
		}
		return err
	}
}

// withJobSlot runs executor while holding one of the slots limiting how many
// jobs execute at the same time (--concurrent-jobs). The slot is taken only
// once the job is ready to run, so jobs waiting for a concurrency group do
// not occupy slots other workflow runs need to make progress.
func (rc *RunContext) withJobSlot(ctx context.Context, executor common.Executor) error {
	if rc.jobSlots == nil {
		return executor(ctx)
	}
	select {
	case rc.jobSlots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-rc.jobSlots }()
	return executor(ctx)
}

// newJobCancelContext returns a context that gracefully cancels this job
// when the surrounding cancel context (Ctrl+C), the cancellation of its
// workflow run, or cancel-in-progress of another job fires. It deliberately
// does not inherit from ctx: cancelling it signals the job to stop
// gracefully instead of aborting it.
func (rc *RunContext) newJobCancelContext(ctx context.Context, runState *workflowRunState) (context.Context, context.CancelFunc) {
	jobCancelCtx, cancelJob := context.WithCancel(context.Background())
	var parentDone <-chan struct{}
	if parent := common.JobCancelContext(ctx); parent != nil {
		parentDone = parent.Done()
	}
	var runCancelled <-chan struct{}
	if runState != nil {
		runCancelled = runState.cancelCh
	}
	go func() {
		select {
		case <-parentDone:
			cancelJob()
		case <-runCancelled:
			cancelJob()
		case <-jobCancelCtx.Done():
		}
	}()
	return jobCancelCtx, cancelJob
}

// handleCancelledJob marks the job result 'cancelled' if the job was
// cancelled by its concurrency group or its workflow run (not by the outer
// context) and reports whether it did so
func (rc *RunContext) handleCancelledJob(ctx context.Context, cancelledByGroup bool, group string) bool {
	if ctx.Err() != nil {
		return false
	}
	logger := common.Logger(ctx)
	if cancelledByGroup {
		logger.WithField("jobResult", "cancelled").Infof("\U0001F6AB  Job '%s' was cancelled by concurrency group '%s' with cancel-in-progress", rc.Name, group)
		rc.cancelledResult()
		return true
	}
	if rc.runState != nil && rc.runState.cancelled.Load() {
		logger.WithField("jobResult", "cancelled").Infof("\U0001F6AB  Job '%s' was cancelled because its workflow run was cancelled", rc.Name)
		rc.cancelledResult()
		return true
	}
	return false
}

func (rc *RunContext) cancelledResult() {
	rc.result("cancelled")
	if rc.caller != nil {
		// a cancelled job of a called workflow cancels the calling job
		rc.caller.runContext.result("cancelled")
	}
}
