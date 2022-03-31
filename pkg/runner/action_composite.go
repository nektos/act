package runner

import (
	"context"
	"fmt"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

func execAsComposite(step actionStep, containerActionDir string) common.Executor {
	rc := step.getRunContext()
	action := step.getActionModel()

	return func(ctx context.Context) error {
		eval := rc.NewExpressionEvaluator()

		inputs := make(map[string]interface{})
		for k, input := range action.Inputs {
			inputs[k] = eval.Interpolate(input.Default)
		}
		if step.getStepModel().With != nil {
			for k, v := range step.getStepModel().With {
				inputs[k] = eval.Interpolate(v)
			}
		}

		env := make(map[string]string)
		for k, v := range rc.Env {
			env[k] = eval.Interpolate(v)
		}
		for k, v := range step.getStepModel().Environment() {
			env[k] = eval.Interpolate(v)
		}

		// run with the global config but without secrets
		configCopy := *rc.Config
		configCopy.Secrets = nil

		// create a run context for the composite action to run in
		compositerc := &RunContext{
			Name:    rc.Name,
			JobName: rc.JobName,
			Run: &model.Run{
				JobID: "composite-job",
				Workflow: &model.Workflow{
					Name: rc.Run.Workflow.Name,
					Jobs: map[string]*model.Job{
						"composite-job": {},
					},
				},
			},
			Config:           &configCopy,
			StepResults:      map[string]*model.StepResult{},
			JobContainer:     rc.JobContainer,
			Inputs:           inputs,
			ActionPath:       containerActionDir,
			ActionRepository: rc.ActionRepository,
			ActionRef:        rc.ActionRef,
			Env:              env,
			Masks:            rc.Masks,
			ExtraPath:        rc.ExtraPath,
		}

		ctx = WithCompositeLogger(ctx, &compositerc.Masks)

		// We need to inject a composite RunContext related command
		// handler into the current running job container
		// We need this, to support scoping commands to the composite action
		// executing.
		rawLogger := common.Logger(ctx).WithField("raw_output", true)
		logWriter := common.NewLineWriter(compositerc.commandHandler(ctx), func(s string) bool {
			if rc.Config.LogOutput {
				rawLogger.Infof("%s", s)
			} else {
				rawLogger.Debugf("%s", s)
			}
			return true
		})
		oldout, olderr := compositerc.JobContainer.ReplaceLogWriter(logWriter, logWriter)
		defer (func() {
			rc.JobContainer.ReplaceLogWriter(oldout, olderr)
		})()

		err := runCompositeSteps(ctx, action, compositerc)

		// Map outputs from composite RunContext to job RunContext
		eval = compositerc.NewExpressionEvaluator()
		for outputName, output := range action.Outputs {
			rc.setOutput(ctx, map[string]string{
				"name": outputName,
			}, eval.Interpolate(output.Value))
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
	postSteps := make([]common.Executor, 0)

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

		preSteps = append(preSteps, step.pre())

		steps = append(steps, func(ctx context.Context) error {
			err := step.main()(ctx)
			if err != nil {
				common.Logger(ctx).Errorf("%v", err)
				common.SetJobError(ctx, err)
			} else if ctx.Err() != nil {
				common.Logger(ctx).Errorf("%v", ctx.Err())
				common.SetJobError(ctx, ctx.Err())
			}
			return nil
		})

		postSteps = append([]common.Executor{step.post()}, postSteps...)
	}

	steps = append(steps, common.JobError)
	return &compositeSteps{
		pre: common.NewPipelineExecutor(preSteps...),
		main: func(ctx context.Context) error {
			return common.NewPipelineExecutor(steps...)(common.WithJobErrorContainer(ctx))
		},
		post: common.NewPipelineExecutor(postSteps...),
	}
}

func runCompositeSteps(ctx context.Context, action *model.Action, compositerc *RunContext) error {
	steps := compositerc.compositeExecutor(action)
	var err error
	if steps.pre != nil {
		err = steps.pre(ctx)
	}
	if err == nil {
		err = steps.main(ctx)
	}
	if err == nil && steps.post != nil {
		err = steps.post(ctx)
	}
	return err
}
