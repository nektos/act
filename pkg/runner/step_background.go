package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/exprparser"
	"github.com/nektos/act/pkg/model"
)

// maxConcurrentBackgroundSteps limits how many background steps of a job run
// at the same time, additional background steps queue until a slot is free
const maxConcurrentBackgroundSteps = 10

// backgroundStep tracks one running background step of a job
type backgroundStep struct {
	id      string
	started chan struct{}
	done    chan struct{}
	cancel  context.CancelFunc

	mu        sync.Mutex
	err       error
	cancelled bool
	collected bool
}

func (bs *backgroundStep) queued() bool {
	select {
	case <-bs.started:
		return false
	default:
		return true
	}
}

// markCancelled gracefully terminates the step, a deliberately cancelled
// step does not fail the job
func (bs *backgroundStep) markCancelled() {
	bs.mu.Lock()
	bs.cancelled = true
	bs.mu.Unlock()
	bs.cancel()
}

func (bs *backgroundStep) setError(err error) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if !bs.cancelled {
		bs.err = err
	}
}

// takeError returns the failure of the step the first time it is called, so
// a background step fails the job only at the first `wait` that includes it
func (bs *backgroundStep) takeError() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if bs.cancelled || bs.collected {
		return nil
	}
	bs.collected = true
	return bs.err
}

func (bs *backgroundStep) running() bool {
	select {
	case <-bs.done:
		return false
	default:
		return true
	}
}

// backgroundStepRegistry tracks the background steps of one job execution
type backgroundStepRegistry struct {
	mu    sync.Mutex
	slots chan struct{}
	steps map[string]*backgroundStep
	order []*backgroundStep
}

func newBackgroundStepRegistry() *backgroundStepRegistry {
	return &backgroundStepRegistry{
		slots: make(chan struct{}, maxConcurrentBackgroundSteps),
		steps: map[string]*backgroundStep{},
	}
}

func (r *backgroundStepRegistry) get(id string) *backgroundStep {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.steps[id]
}

func (r *backgroundStepRegistry) all() []*backgroundStep {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*backgroundStep{}, r.order...)
}

// launch starts executor asynchronously as a background step and returns as
// soon as it is registered, the step waits for one of the limited background
// slots before it runs
func (r *backgroundStepRegistry) launch(ctx context.Context, id string, executor common.Executor) error {
	stepCtx, cancel := context.WithCancel(ctx)
	bs := &backgroundStep{id: id, started: make(chan struct{}), done: make(chan struct{}), cancel: cancel}

	r.mu.Lock()
	if _, exists := r.steps[id]; exists {
		r.mu.Unlock()
		cancel()
		return fmt.Errorf("background step id '%s' is not unique, wait and cancel steps could not tell the steps apart", id)
	}
	r.steps[id] = bs
	r.order = append(r.order, bs)
	r.mu.Unlock()

	go func() {
		defer close(bs.done)
		defer cancel()
		select {
		case r.slots <- struct{}{}:
		default:
			common.Logger(ctx).Infof("Background step '%s' is queued, at most %d background steps run concurrently", id, maxConcurrentBackgroundSteps)
			select {
			case r.slots <- struct{}{}:
			case <-stepCtx.Done():
				bs.setError(stepCtx.Err())
				return
			}
		}
		close(bs.started)
		defer func() { <-r.slots }()
		bs.setError(executor(stepCtx))
	}()
	return nil
}

func (r *backgroundStepRegistry) waitFor(ctx context.Context, bs *backgroundStep) error {
	select {
	case <-bs.done:
		return bs.takeError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// cancelRemaining stops background steps that are still running when the job
// ends, like services started with `background: true` that were never waited
// for, their results do not affect the job result
func (r *backgroundStepRegistry) cancelRemaining() common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		for _, bs := range r.all() {
			if bs.running() {
				logger.Infof("\U0001F6D1  Stopping background step '%s' still running at the end of the job", bs.id)
				bs.markCancelled()
			}
		}
		for _, bs := range r.all() {
			select {
			case <-bs.done:
			case <-time.After(time.Minute):
				logger.Errorf("Background step '%s' did not stop in time", bs.id)
			}
		}
		return nil
	}
}

// expandParallelStepGroups desugars every `parallel` step group into
// background steps followed by an implicit `wait` for the whole group
func expandParallelStepGroups(steps []*model.Step) ([]*model.Step, error) {
	expanded := make([]*model.Step, 0, len(steps))
	for _, stepModel := range steps {
		if stepModel == nil || stepModel.Type() != model.StepTypeParallel {
			expanded = append(expanded, stepModel)
			continue
		}
		group := stepModel.ParallelSteps()
		if len(group) == 0 {
			return nil, fmt.Errorf("parallel step group '%s' contains no steps", stepModel.String())
		}
		ids := make([]string, 0, len(group))
		for i, child := range group {
			if child == nil {
				return nil, fmt.Errorf("invalid step %d in parallel step group '%s': missing run or uses key", i, stepModel.String())
			}
			if child.ID == "" {
				child.ID = fmt.Sprintf("%d", len(expanded))
			}
			child.Background = "true"
			ids = append(ids, child.ID)
			expanded = append(expanded, child)
		}
		wait := &model.Step{
			ID:   stepModel.ID,
			Name: fmt.Sprintf("Wait for parallel steps (%s)", strings.Join(ids, ", ")),
		}
		wait.RawWait.Kind = yaml.SequenceNode
		wait.RawWait.Tag = "!!seq"
		for _, id := range ids {
			wait.RawWait.Content = append(wait.RawWait.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id})
		}
		expanded = append(expanded, wait)
	}
	return expanded, nil
}

// newMainStepExecutor returns the main stage executor of a regular step.
// Steps with `background: true` are only started and the job continues with
// the next step right away, their failures surface at the next `wait` or
// `wait-all` step that includes them.
func newMainStepExecutor(rc *RunContext, registry *backgroundStepRegistry, stepModel *model.Step, stepExec common.Executor, setJobError func(context.Context, error) error) common.Executor {
	if stepModel.IsBackground() {
		backgroundExec := useStepLogger(rc, stepModel, stepStageMain, stepExec)
		stepID := stepModel.ID
		stepName := stepModel.String()
		return func(ctx context.Context) error {
			common.Logger(ctx).Infof("⏩ Starting background step '%s'", stepName)
			if err := registry.launch(ctx, stepID, backgroundExec); err != nil {
				_ = setJobError(ctx, err)
			}
			return nil
		}
	}
	return useStepLogger(rc, stepModel, stepStageMain, func(ctx context.Context) error {
		err := stepExec(ctx)
		if err != nil {
			_ = setJobError(ctx, err)
		} else if ctx.Err() != nil {
			_ = setJobError(ctx, ctx.Err())
		}
		return nil
	})
}

// newControlStepExecutor runs a `wait`, `wait-all` or `cancel` step. These
// steps always run, even when a previous step failed, and do not support the
// `if` conditional.
func newControlStepExecutor(rc *RunContext, registry *backgroundStepRegistry, stepModel *model.Step, setJobError func(context.Context, error) error) common.Executor {
	controlExec := newBackgroundControlExecutor(rc, registry, stepModel)
	return useStepLogger(rc, stepModel, stepStageMain, func(ctx context.Context) error {
		err := controlExec(ctx)
		if err != nil {
			_ = setJobError(ctx, err)
		} else if ctx.Err() != nil {
			_ = setJobError(ctx, ctx.Err())
		}
		return nil
	})
}

func newBackgroundControlExecutor(rc *RunContext, registry *backgroundStepRegistry, stepModel *model.Step) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)

		stepResult := &model.StepResult{
			Outcome:    model.StepStatusSuccess,
			Conclusion: model.StepStatusSuccess,
			Outputs:    make(map[string]string),
		}
		lock := rc.stepStateLock()
		dataLock := rc.stateDataLock()
		lock.Lock()
		dataLock.Lock()
		rc.StepResults[stepModel.ID] = stepResult
		dataLock.Unlock()
		lock.Unlock()

		logger.Infof("⭐ Run %s", stepModel.String())

		var err error
		switch stepModel.Type() {
		case model.StepTypeWait:
			err = runWaitStep(ctx, registry, stepModel)
		case model.StepTypeWaitAll:
			err = runWaitAllStep(ctx, registry)
		case model.StepTypeCancel:
			err = runCancelStep(ctx, registry, stepModel)
		default:
			err = fmt.Errorf("step '%s' is not a background control step", stepModel.ID)
		}

		if err == nil {
			logger.WithField("stepResult", stepResult.Outcome).Infof("  ✅  Success - %s", stepModel.String())
			return nil
		}

		dataLock.Lock()
		stepResult.Outcome = model.StepStatusFailure
		dataLock.Unlock()
		continueOnError, parseErr := evalStepContinueOnError(ctx, rc, stepModel)
		if parseErr != nil {
			dataLock.Lock()
			stepResult.Conclusion = model.StepStatusFailure
			dataLock.Unlock()
			return parseErr
		}
		if continueOnError {
			logger.Infof("Failed but continue next step")
			logger.WithField("stepResult", stepResult.Outcome).Infof("  ❌  Failure - %s", stepModel.String())
			return nil
		}
		dataLock.Lock()
		stepResult.Conclusion = model.StepStatusFailure
		dataLock.Unlock()
		logger.WithField("stepResult", stepResult.Outcome).Infof("  ❌  Failure - %s", stepModel.String())
		return err
	}
}

func runWaitStep(ctx context.Context, registry *backgroundStepRegistry, stepModel *model.Step) error {
	ids := stepModel.Wait()
	if len(ids) == 0 {
		return fmt.Errorf("wait step '%s' does not reference any background step", stepModel.ID)
	}
	var errs []error
	for _, id := range ids {
		bs := registry.get(id)
		if bs == nil {
			errs = append(errs, fmt.Errorf("'%s' is not a background step", id))
			continue
		}
		if bs.queued() {
			common.Logger(ctx).Warnf("Background step '%s' has not started yet, it is queued until one of the %d background slots is free", id, maxConcurrentBackgroundSteps)
		}
		common.Logger(ctx).Infof("⏸  Waiting for background step '%s'", id)
		if err := registry.waitFor(ctx, bs); err != nil {
			errs = append(errs, fmt.Errorf("background step '%s' failed: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func runWaitAllStep(ctx context.Context, registry *backgroundStepRegistry) error {
	var errs []error
	for _, bs := range registry.all() {
		common.Logger(ctx).Infof("⏸  Waiting for background step '%s'", bs.id)
		if err := registry.waitFor(ctx, bs); err != nil {
			errs = append(errs, fmt.Errorf("background step '%s' failed: %w", bs.id, err))
		}
	}
	return errors.Join(errs...)
}

func runCancelStep(ctx context.Context, registry *backgroundStepRegistry, stepModel *model.Step) error {
	bs := registry.get(stepModel.Cancel)
	if bs == nil {
		return fmt.Errorf("'%s' is not a background step", stepModel.Cancel)
	}
	if !bs.running() {
		// cancelling a step that already finished must not suppress a
		// failure it recorded, a later wait still reports it
		common.Logger(ctx).Infof("Background step '%s' already finished, nothing to cancel", bs.id)
		return nil
	}
	common.Logger(ctx).Infof("\U0001F6D1  Cancelling background step '%s'", bs.id)
	bs.markCancelled()
	select {
	case <-bs.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func evalStepContinueOnError(ctx context.Context, rc *RunContext, stepModel *model.Step) (bool, error) {
	if stepModel.RawContinueOnError == "" {
		return false, nil
	}
	continueOnError, err := EvalBool(ctx, rc.NewExpressionEvaluator(ctx), stepModel.RawContinueOnError, exprparser.DefaultStatusCheckNone)
	if err != nil {
		return false, fmt.Errorf("  ❌  Error in continue-on-error-expression: \"continue-on-error: %s\" (%s)", stepModel.RawContinueOnError, err)
	}
	return continueOnError, nil
}
