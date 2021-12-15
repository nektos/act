package runner

import (
	"context"
	"fmt"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/common/executor"
	"github.com/nektos/act/pkg/common/logger"
	"github.com/nektos/act/pkg/model"
)

type jobInfo interface {
	matrix() map[string]interface{}
	steps() []*model.Step
	startContainer() executor.Executor
	stopContainer() executor.Executor
	closeContainer() executor.Executor
	interpolateOutputs() executor.Executor
	result(result string)
}

func newJobExecutor(info jobInfo, sf stepFactory, rc *RunContext) executor.Executor {
	steps := make([]executor.Executor, 0)
	preSteps := make([]executor.Executor, 0)
	postSteps := make([]executor.Executor, 0)

	steps = append(steps, func(ctx context.Context) error {
		if len(info.matrix()) > 0 {
			logger.Logger(ctx).Infof("\U0001F9EA  Matrix: %v", info.matrix())
		}
		return nil
	})

	infoSteps := info.steps()

	if len(infoSteps) == 0 {
		return executor.NewDebugExecutor("No steps found")
	}

	preSteps = append(preSteps, info.startContainer())

	for i, stepModel := range infoSteps {
		if stepModel.ID == "" {
			stepModel.ID = fmt.Sprintf("%d", i)
		}

		step, err := sf.newStep(stepModel, rc)

		if err != nil {
			return executor.NewErrorExecutor(err)
		}

		preSteps = append(preSteps, step.pre())

		stepExec := step.main()
		steps = append(steps, func(ctx context.Context) error {
			stepName := stepModel.String()
			return (func(ctx context.Context) error {
				err := stepExec(ctx)
				if err != nil {
					logger.Logger(ctx).Errorf("%v", err)
					common.SetJobError(ctx, err)
				} else if ctx.Err() != nil {
					logger.Logger(ctx).Errorf("%v", ctx.Err())
					common.SetJobError(ctx, ctx.Err())
				}
				return nil
			})(logger.WithStepLogger(ctx, stepName))
		})

		postSteps = append([]executor.Executor{step.post()}, postSteps...)
	}

	postSteps = append(postSteps, func(ctx context.Context) error {
		jobError := common.JobError(ctx)
		if jobError != nil {
			info.result("failure")
		} else {
			err := info.stopContainer()(ctx)
			if err != nil {
				return err
			}
			info.result("success")
		}

		return nil
	})

	pipeline := make([]executor.Executor, 0)
	pipeline = append(pipeline, preSteps...)
	pipeline = append(pipeline, steps...)
	pipeline = append(pipeline, postSteps...)

	return executor.NewPipelineExecutor(pipeline...).Finally(info.interpolateOutputs()).Finally(info.closeContainer())
}
