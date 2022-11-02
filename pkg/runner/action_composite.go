package runner

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

func evaluateCompositeInputAndEnv(ctx context.Context, parent *RunContext, step actionStep) map[string]string {
	env := make(map[string]string)
	stepEnv := *step.getEnv()
	for k, v := range stepEnv {
		// do not set current inputs into composite action
		// the required inputs are added in the second loop
		if !strings.HasPrefix(k, "INPUT_") {
			env[k] = v
		}
	}

	ee := parent.NewStepExpressionEvaluator(ctx, step)

	for inputID, input := range step.getActionModel().Inputs {
		envKey := regexp.MustCompile("[^A-Z0-9-]").ReplaceAllString(strings.ToUpper(inputID), "_")
		envKey = fmt.Sprintf("INPUT_%s", strings.ToUpper(envKey))

		// lookup if key is defined in the step but the the already
		// evaluated value from the environment
		_, defined := step.getStepModel().With[inputID]
		if value, ok := stepEnv[envKey]; defined && ok {
			env[envKey] = value
		} else {
			// defaults could contain expressions
			env[envKey] = ee.Interpolate(ctx, input.Default)
		}
	}

	return env
}

func newCompositeRunContext(ctx context.Context, parent *RunContext, step actionStep, actionPath string) *RunContext {
	env := evaluateCompositeInputAndEnv(ctx, parent, step)

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
		Config:       &configCopy,
		StepResults:  map[string]*model.StepResult{},
		JobContainer: parent.JobContainer,
		ActionPath:   actionPath,
		Env:          env,
		Masks:        parent.Masks,
		ExtraPath:    parent.ExtraPath,
		Parent:       parent,
	}
	compositerc.ExprEval = compositerc.NewExpressionEvaluator(ctx)

	return compositerc
}

func execAsComposite(step actionStep) common.Executor {
	rc := step.getRunContext()
	action := step.getActionModel()

	return func(ctx context.Context) error {
		compositeRC := step.getCompositeRunContext(ctx)

		steps := step.getCompositeSteps()

		ctx = WithCompositeLogger(ctx, &compositeRC.Masks)

		err := steps.main(ctx)

		// Map outputs from composite RunContext to job RunContext
		eval := compositeRC.NewExpressionEvaluator(ctx)
		for outputName, output := range action.Outputs {
			rc.setOutput(ctx, map[string]string{
				"name": outputName,
			}, eval.Interpolate(ctx, output.Value))
		}

		rc.Masks = append(rc.Masks, compositeRC.Masks...)
		rc.ExtraPath = compositeRC.ExtraPath

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
		stepPre := rc.newCompositeCommandExecutor(step.pre())
		preSteps = append(preSteps, newCompositeStepLogExecutor(stepPre, stepID))

		steps = append(steps, func(ctx context.Context) error {
			ctx = WithCompositeStepLogger(ctx, stepID)
			logger := common.Logger(ctx)
			err := rc.newCompositeCommandExecutor(step.main())(ctx)

			if err != nil {
				logger.Errorf("%v", err)
				common.SetJobError(ctx, err)
			} else if ctx.Err() != nil {
				logger.Errorf("%v", ctx.Err())
				common.SetJobError(ctx, ctx.Err())
			}
			return nil
		})

		// run the post executor in reverse order
		if postExecutor != nil {
			stepPost := rc.newCompositeCommandExecutor(step.post())
			postExecutor = newCompositeStepLogExecutor(stepPost.Finally(postExecutor), stepID)
		} else {
			stepPost := rc.newCompositeCommandExecutor(step.post())
			postExecutor = newCompositeStepLogExecutor(stepPost, stepID)
		}
	}

	steps = append(steps, common.JobError)
	return &compositeSteps{
		pre: func(ctx context.Context) error {
			return common.NewPipelineExecutor(preSteps...)(common.WithJobErrorContainer(ctx))
		},
		main: func(ctx context.Context) error {
			return common.NewPipelineExecutor(steps...)(common.WithJobErrorContainer(ctx))
		},
		post: postExecutor,
	}
}

func (rc *RunContext) newCompositeCommandExecutor(executor common.Executor) common.Executor {
	return func(ctx context.Context) error {
		ctx = WithCompositeLogger(ctx, &rc.Masks)

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

		return executor(ctx)
	}
}

func newCompositeStepLogExecutor(runStep common.Executor, stepID string) common.Executor {
	return func(ctx context.Context) error {
		ctx = WithCompositeStepLogger(ctx, stepID)
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
