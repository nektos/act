package runner

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

// errCancelledByConcurrencyGroup is returned by acquire when the owner of the
// request was cancelled by another job entering one of its concurrency groups
// with cancel-in-progress enabled.
var errCancelledByConcurrencyGroup = errors.New("cancelled by another job in the same concurrency group")

// concurrencySpec describes a single concurrency group requirement of a job.
// Jobs sharing the same owner may hold the group at the same time. This lets
// all jobs of one workflow run share the workflow level concurrency group,
// while job level groups use a unique owner per job so they serialize.
type concurrencySpec struct {
	group            string
	owner            string
	cancelInProgress bool
}

type concurrencyGroup struct {
	owner string
	// holders maps an id per acquisition to the cancel function of the
	// holding job, so cancel-in-progress can cancel all current holders
	holders map[int64]context.CancelFunc
	// released is closed (and the group dropped) once the last holder
	// releases the group, waking up all waiting jobs
	released chan struct{}
}

// concurrencyManager serializes jobs that share a concurrency group. A single
// instance is shared between a runner and all reusable workflow runners
// spawned from it, so called workflows take part in the same groups.
type concurrencyManager struct {
	mu        sync.Mutex
	holderSeq int64
	ownerSeq  atomic.Int64
	groups    map[string]*concurrencyGroup
	// cancelledOwners records owners that were cancelled via
	// cancel-in-progress; their remaining jobs are cancelled instead of run,
	// approximating GitHub cancelling the whole workflow run
	cancelledOwners map[string]bool
}

func newConcurrencyManager() *concurrencyManager {
	return &concurrencyManager{
		groups:          map[string]*concurrencyGroup{},
		cancelledOwners: map[string]bool{},
	}
}

// newOwner returns a new unique owner identity for a workflow run or a job
func (m *concurrencyManager) newOwner(kind string) string {
	return fmt.Sprintf("%s-%d", kind, m.ownerSeq.Add(1))
}

// acquire blocks until all requested groups are held by the calling job and
// returns a function releasing them again. cancelSelf is invoked if another
// job cancels this one via cancel-in-progress.
func (m *concurrencyManager) acquire(ctx context.Context, cancelSelf context.CancelFunc, specs ...concurrencySpec) (func(), error) {
	ordered := make([]concurrencySpec, len(specs))
	copy(ordered, specs)
	// acquire groups in a stable order to avoid deadlocks between jobs
	// requesting the same groups in a different order
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].group < ordered[j].group })

	releases := make([]func(), 0, len(ordered))
	releaseAll := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}

	for _, spec := range ordered {
		release, err := m.acquireGroup(ctx, spec, cancelSelf)
		if err != nil {
			releaseAll()
			return nil, err
		}
		releases = append(releases, release)
	}
	return releaseAll, nil
}

func (m *concurrencyManager) acquireGroup(ctx context.Context, spec concurrencySpec, cancelSelf context.CancelFunc) (func(), error) {
	logger := common.Logger(ctx)
	waiting := false
	m.mu.Lock()
	for {
		if m.cancelledOwners[spec.owner] {
			m.mu.Unlock()
			return nil, errCancelledByConcurrencyGroup
		}
		group := m.groups[spec.group]
		if group == nil {
			group = &concurrencyGroup{
				holders:  map[int64]context.CancelFunc{},
				released: make(chan struct{}),
			}
			m.groups[spec.group] = group
		}
		if len(group.holders) == 0 || group.owner == spec.owner {
			group.owner = spec.owner
			m.holderSeq++
			id := m.holderSeq
			group.holders[id] = cancelSelf
			m.mu.Unlock()
			if waiting {
				logger.Infof("Acquired concurrency group '%s'", spec.group)
			}
			return func() { m.release(spec.group, id) }, nil
		}
		if spec.cancelInProgress {
			// mark the current owner as cancelled and cancel all its jobs
			// holding the group; the group is re-acquired once they release it
			m.cancelledOwners[group.owner] = true
			logger.Infof("Cancelling in-progress jobs in concurrency group '%s'", spec.group)
			for _, cancel := range group.holders {
				cancel()
			}
		}
		if !waiting {
			logger.Infof("Waiting for concurrency group '%s'", spec.group)
			waiting = true
		}
		released := group.released
		m.mu.Unlock()
		select {
		case <-released:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		m.mu.Lock()
	}
}

func (m *concurrencyManager) release(groupName string, id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	group := m.groups[groupName]
	if group == nil {
		return
	}
	delete(group.holders, id)
	if len(group.holders) == 0 {
		delete(m.groups, groupName)
		close(group.released)
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

// concurrencySpecs evaluates the workflow and job level concurrency groups
// that apply to this job
func (rc *RunContext) concurrencySpecs(ctx context.Context) []concurrencySpec {
	specs := make([]concurrencySpec, 0, 2)
	ee := rc.NewExpressionEvaluator(ctx)
	if spec, ok := evaluateConcurrency(ctx, ee, rc.Run.Job().Concurrency(), rc.jobConcurrencyOwner); ok {
		specs = append(specs, spec)
	}
	if spec, ok := evaluateConcurrency(ctx, ee, rc.Run.Workflow.Concurrency(), rc.workflowConcurrencyOwner); ok {
		// if the job level group has the same name the job would wait for a
		// group it already holds, so only keep the stricter job level spec
		if len(specs) == 0 || specs[0].group != spec.group {
			specs = append(specs, spec)
		}
	}
	return specs
}

func evaluateConcurrency(ctx context.Context, ee ExpressionEvaluator, concurrency *model.Concurrency, owner string) (concurrencySpec, bool) {
	if concurrency == nil || owner == "" {
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
	return concurrencySpec{group: group, owner: owner, cancelInProgress: cancelInProgress}, true
}

// withConcurrency wraps a job executor so it holds the concurrency groups of
// the job while running. Jobs cancelled by cancel-in-progress of another job
// finish with result 'cancelled' without failing the plan.
func (rc *RunContext) withConcurrency(executor common.Executor) common.Executor {
	return func(ctx context.Context) error {
		specs := rc.concurrencySpecs(ctx)
		if len(specs) == 0 {
			return executor(ctx)
		}

		logger := common.Logger(ctx)

		// gracefully cancel this job when either the surrounding cancel
		// context (Ctrl+C) or cancel-in-progress of another job fires
		jobCancelCtx, cancelJob := context.WithCancel(context.Background())
		defer cancelJob()
		if parent := common.JobCancelContext(ctx); parent != nil {
			go func() {
				select {
				case <-parent.Done():
					cancelJob()
				case <-jobCancelCtx.Done():
				}
			}()
		}

		var cancelledByGroup atomic.Bool
		cancelSelf := func() {
			cancelledByGroup.Store(true)
			cancelJob()
		}

		release, err := rc.Config.concurrency().acquire(ctx, cancelSelf, specs...)
		if err != nil {
			if errors.Is(err, errCancelledByConcurrencyGroup) && ctx.Err() == nil {
				logger.WithField("jobResult", "cancelled").Infof("\U0001F6AB  Job '%s' was cancelled by concurrency group before it started", rc.Name)
				rc.cancelledResult()
				return nil
			}
			return err
		}
		defer release()

		err = executor(common.WithJobCancelContext(ctx, jobCancelCtx))
		if cancelledByGroup.Load() && ctx.Err() == nil {
			logger.WithField("jobResult", "cancelled").Infof("\U0001F6AB  Job '%s' was cancelled by a concurrency group with cancel-in-progress", rc.Name)
			rc.cancelledResult()
			return nil
		}
		return err
	}
}

func (rc *RunContext) cancelledResult() {
	rc.result("cancelled")
	if rc.caller != nil {
		// a cancelled job of a called workflow cancels the calling job
		rc.caller.runContext.result("cancelled")
	}
}
