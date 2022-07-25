package runner

import (
	"context"
	"fmt"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

func evaluteCompositeInputAndEnv(ctx context.Context, parent *RunContext, step actionStep) (inputs map[string]interface{}, env map[string]string) {
	eval := parent.NewExpressionEvaluator(ctx)

	inputs = make(map[string]interface{})
	for k, input := range step.getActionModel().Inputs {
		inputs[k] = eval.Interpolate(ctx, input.Default)
	}
	if step.getStepModel().With != nil {
		for k, v := range step.getStepModel().With {
			inputs[k] = eval.Interpolate(ctx, v)
		}
	}

	env = make(map[string]string)
	for k, v := range parent.Env {
		env[k] = eval.Interpolate(ctx, v)
	}
	for k, v := range step.getStepModel().Environment() {
		env[k] = eval.Interpolate(ctx, v)
	}

	return inputs, env
}

func newCompositeRunContext(ctx context.Context, parent *RunContext, step actionStep, actionPath string) *RunContext {
	inputs, env := evaluteCompositeInputAndEnv(ctx, parent, step)

	// run with the global config but without secrets
	configCopy := *(parent.Config)
	configCopy.Secrets = nil

	// create a run context for the composite action to run in
	compositerc := &RunContext{
		Name:    parent.Name,
		JobName: parent.JobName,
		Run: &model.Run{
			JobID: "composite-job",
			Workflow: &model.Workflow{
				Name: parent.Run.Workflow.Name,
				Jobs: map[string]*model.Job{
					"composite-job": {},
				},
			},
		},
		Config:           &configCopy,
		StepResults:      map[string]*model.StepResult{},
		JobContainer:     parent.JobContainer,
		Inputs:           inputs,
		ActionPath:       actionPath,
		ActionRepository: parent.ActionRepository,
		ActionRef:        parent.ActionRef,
		Env:              env,
		Masks:            parent.Masks,
		ExtraPath:        parent.ExtraPath,
		Parent:           parent,
	}

	return compositerc
}

// This updates a composite context inputs, env and masks.
// This is needed to re-evalute/update that context between pre/main/post steps.
// Some of the inputs/env may requires the results of in-between steps.
func (rc *RunContext) updateCompositeRunContext(ctx context.Context, parent *RunContext, step actionStep) {
	inputs, env := evaluteCompositeInputAndEnv(ctx, parent, step)

	rc.Inputs = inputs
	rc.Env = env
	rc.Masks = append(rc.Masks, parent.Masks...)
}

func execAsComposite(step actionStep) common.Executor {
	rc := step.getRunContext()
	action := step.getActionModel()

	return func(ctx context.Context) error {
		compositerc := step.getCompositeRunContext(ctx)

		steps := step.getCompositeSteps()

		ctx = WithCompositeLogger(ctx, &compositerc.Masks)

		compositerc.updateCompositeRunContext(ctx, rc, step)
		err := steps.main(ctx)

		// Map outputs from composite RunContext to job RunContext
		eval := compositerc.NewExpressionEvaluator(ctx)
		for outputName, output := range action.Outputs {
			rc.setOutput(ctx, map[string]string{
				"name": outputName,
			}, eval.Interpolate(ctx, output.Value))
		}

		rc.Masks = append(rc.Masks, compositerc.Masks...)
		rc.ExtraPath = compositerc.ExtraPath

		return err
	}
}

type compositeSteps struct {
	pre  common.Executor
	main common.Executor
	post common.Executor
}

// Executor returns a pipeline executor for all the steps in the job
func (rc *RunContext) compositeExecutor(action *model.Action) *compositeSteps {
	steps := make([]common.Executor, 0)
	preSteps := make([]common.Executor, 0)
	var postExecutor common.Executor

	sf := &stepFactoryImpl{}

	for i, step := range action.Runs.Steps {
		if step.ID == "" {
			step.ID = fmt.Sprintf("%d", i)
		}

		// create a copy of the step, since this composite action could
		// run multiple times and we might modify the instance
		stepcopy := step

		step, err := sf.newStep(&stepcopy, rc)
		if err != nil {
			return &compositeSteps{
				main: common.NewErrorExecutor(err),
			}
		}

		stepID := step.getStepModel().ID
		stepPre := step.pre()
		preSteps = append(preSteps, newCompositeStepLogExecutor(rc, stepPre, stepID))

		steps = append(steps, newCompositeStepLogExecutor(rc, step.main(), stepID))

		// run the post executor in reverse order
		if postExecutor != nil {
			stepPost := step.post()
			postExecutor = newCompositeStepLogExecutor(rc, stepPost, stepID)
			stepPost.Finally(postExecutor)
		} else {
			stepPost := step.post()
			postExecutor = newCompositeStepLogExecutor(rc, stepPost, stepID)
		}
	}

	steps = append(steps, common.JobError)
	return &compositeSteps{
		pre: rc.newCompositeCommandExecutor(func(ctx context.Context) error {
			return common.NewPipelineExecutor(preSteps...)(common.WithJobErrorContainer(ctx))
		}),
		main: rc.newCompositeCommandExecutor(func(ctx context.Context) error {
			return common.NewPipelineExecutor(steps...)(common.WithJobErrorContainer(ctx))
		}),
		post: rc.newCompositeCommandExecutor(postExecutor),
	}
}

func (rc *RunContext) newCompositeCommandExecutor(executor common.Executor) common.Executor {
	return func(ctx context.Context) error {
		ctx = WithCompositeLogger(ctx, &rc.Masks)

		return executor(ctx)
	}
}

func newCompositeStepLogExecutor(rc *RunContext, runStep common.Executor, stepID string) common.Executor {
	return func(ctx context.Context) error {
		ctx = WithCompositeStepLogger(ctx, stepID)

		// We need to inject a composite RunContext related command
		// handler into the current running job container
		// We need this, to support scoping commands to the composite action
		// executing.
		rawLogger := common.Logger(ctx).WithField("raw_output", true)
		logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) bool {
			if rc.Config.LogOutput {
				rawLogger.Infof("%s", s)
			} else {
				rawLogger.Debugf("%s", s)
			}
			return true
		})

		oldout, olderr := rc.JobContainer.ReplaceLogWriter(logWriter, logWriter)
		defer rc.JobContainer.ReplaceLogWriter(oldout, olderr)

		logger := common.Logger(ctx)
		err := runStep(ctx)
		if err != nil {
			logger.Errorf("%v", err)
			common.SetJobError(ctx, err)
		} else if ctx.Err() != nil {
			logger.Errorf("%v", ctx.Err())
			common.SetJobError(ctx, ctx.Err())
		}
		return nil
	}
}
