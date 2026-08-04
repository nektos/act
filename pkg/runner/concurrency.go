package runner

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

// errCancelledByConcurrencyGroup is returned by acquire when the request was
// cancelled instead of run: either it was superseded by a newer request in
// the same group (GitHub cancels the previously pending run) or the job/run
// itself was cancelled while waiting.
var errCancelledByConcurrencyGroup = errors.New("cancelled by the concurrency group")

// concurrencySpec is an evaluated `concurrency` configuration
type concurrencySpec struct {
	group            string
	cancelInProgress bool
}

// concurrencyWaiter represents one workflow run or job holding or waiting
// for a concurrency group
type concurrencyWaiter struct {
	// cancelSelf gracefully cancels the holder when a new request arrives
	// with cancel-in-progress
	cancelSelf func()
	// promoted is closed when the waiter becomes the holder of the group
	promoted chan struct{}
	// superseded is closed when a newer pending request replaces this one,
	// the superseded request is cancelled without having run
	superseded chan struct{}
}

type concurrencyGroup struct {
	holder  *concurrencyWaiter
	pending *concurrencyWaiter
}

// concurrencyManager implements GitHub's concurrency group queueing: one
// holder runs at a time, at most one request is pending and a newer request
// supersedes (cancels) the previously pending one. A single instance is
// shared between a runner and all reusable workflow runners spawned from it.
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

	m.mu.Lock()
	group := m.groups[spec.group]
	if group == nil {
		group = &concurrencyGroup{}
		m.groups[spec.group] = group
	}
	if group.holder == nil {
		group.holder = me
		m.mu.Unlock()
		return func() { m.release(spec.group, me) }, nil
	}
	if spec.cancelInProgress && group.holder.cancelSelf != nil {
		logger.Infof("Cancelling the run in progress in concurrency group '%s'", spec.group)
		group.holder.cancelSelf()
	}
	if group.pending != nil {
		// GitHub keeps at most one pending run per group: the previously
		// pending run is cancelled and replaced by the newer request
		logger.Infof("Superseding the run pending in concurrency group '%s'", spec.group)
		close(group.pending.superseded)
	}
	group.pending = me
	m.mu.Unlock()

	logger.Infof("Waiting for concurrency group '%s'", spec.group)

	var cancelDone <-chan struct{}
	if cancelCtx != nil {
		cancelDone = cancelCtx.Done()
	}
	select {
	case <-me.promoted:
		logger.Infof("Acquired concurrency group '%s'", spec.group)
		return func() { m.release(spec.group, me) }, nil
	case <-me.superseded:
		return nil, errCancelledByConcurrencyGroup
	case <-cancelDone:
		m.removePending(spec.group, me)
		return nil, errCancelledByConcurrencyGroup
	case <-ctx.Done():
		m.removePending(spec.group, me)
		return nil, ctx.Err()
	}
}

func (m *concurrencyManager) release(groupName string, me *concurrencyWaiter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	group := m.groups[groupName]
	if group == nil || group.holder != me {
		return
	}
	if group.pending != nil {
		group.holder = group.pending
		group.pending = nil
		close(group.holder.promoted)
		return
	}
	delete(m.groups, groupName)
}

func (m *concurrencyManager) removePending(groupName string, me *concurrencyWaiter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if group := m.groups[groupName]; group != nil && group.pending == me {
		group.pending = nil
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
	held := map[string]bool{group: true}
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

func evaluateConcurrency(ctx context.Context, ee ExpressionEvaluator, concurrency *model.Concurrency) (concurrencySpec, bool) {
	if concurrency == nil {
		return concurrencySpec{}, false
	}
	group := strings.TrimSpace(ee.Interpolate(ctx, concurrency.Group))
	if group == "" {
		common.Logger(ctx).Warnf("Concurrency group evaluated to an empty string, ignoring the concurrency settings")
		return concurrencySpec{}, false
	}
	cancelInProgress := false
	if val := strings.TrimSpace(ee.Interpolate(ctx, concurrency.CancelInProgress)); val != "" {
		var err error
		if cancelInProgress, err = strconv.ParseBool(val); err != nil {
			common.Logger(ctx).Errorf("Failed to parse 'cancel-in-progress' value '%s': %v", val, err)
			cancelInProgress = false
		}
	}
	return concurrencySpec{group: group, cancelInProgress: cancelInProgress}, true
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

		spec, hasSpec := evaluateConcurrency(ctx, rc.NewExpressionEvaluator(ctx), rc.Run.Job().Concurrency())
		if hasSpec && heldConcurrencyGroups(ctx)[spec.group] {
			// the group is already held by this run or a calling workflow,
			// acquiring it again would deadlock on our own caller
			logger.Debugf("Concurrency group '%s' is already held by this workflow run or its caller", spec.group)
			hasSpec = false
		}

		if !hasSpec && runState == nil {
			return executor(ctx)
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
				if errors.Is(err, errCancelledByConcurrencyGroup) && ctx.Err() == nil {
					logger.WithField("jobResult", "cancelled").Infof("\U0001F6AB  Job '%s' was cancelled by concurrency group '%s' before it started", rc.Name, spec.group)
					rc.cancelledResult()
					return nil
				}
				return err
			}
			defer release()
			ctx = withHeldConcurrencyGroup(ctx, spec.group)
		}

		err := executor(common.WithJobCancelContext(ctx, jobCancelCtx))
		jobCompleted.Store(true)
		if rc.handleCancelledJob(ctx, cancelledByGroup.Load(), spec.group) {
			return nil
		}
		return err
	}
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
