package runner

import (
	"context"
	"fmt"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

type jobInfo interface {
	matrix() map[string]interface{}
	steps() []*model.Step
	startContainer() common.Executor
	stopContainer() common.Executor
	closeContainer() common.Executor
	interpolateOutputs() common.Executor
	result(result string)
}

func newJobExecutor(info jobInfo, sf stepFactory, rc *RunContext) common.Executor {
	steps := make([]common.Executor, 0)
	preSteps := make([]common.Executor, 0)
	postSteps := make([]common.Executor, 0)

	steps = append(steps, func(ctx context.Context) error {
		if len(info.matrix()) > 0 {
			common.Logger(ctx).Infof("\U0001F9EA  Matrix: %v", info.matrix())
		}
		return nil
	})

	infoSteps := info.steps()

	if len(infoSteps) == 0 {
		return common.NewDebugExecutor("No steps found")
	}

	preSteps = append(preSteps, info.startContainer())

	for i, stepModel := range infoSteps {
		if stepModel.ID == "" {
			stepModel.ID = fmt.Sprintf("%d", i)
		}

		step, err := sf.newStep(stepModel, rc)

		if err != nil {
			return common.NewErrorExecutor(err)
		}

		preSteps = append(preSteps, step.pre())

		stepExec := step.main()
		steps = append(steps, func(ctx context.Context) error {
			err := stepExec(ctx)
			if err != nil {
				common.Logger(ctx).Errorf("%v", err)
				common.SetJobError(ctx, err)
			} else if ctx.Err() != nil {
				common.Logger(ctx).Errorf("%v", ctx.Err())
				common.SetJobError(ctx, ctx.Err())
			}
			return nil
		})

		postSteps = append(postSteps, step.post())
	}
	postSteps = append(postSteps, func(ctx context.Context) error {
		err := info.stopContainer()(ctx)
		if err != nil {
			return err
		}

		jobError := common.JobError(ctx)
		if jobError != nil {
			info.result("failure")
		} else {
			info.result("success")
		}

		return nil
	})

	pipeline := make([]common.Executor, 0)
	pipeline = append(pipeline, preSteps...)
	pipeline = append(pipeline, steps...)
	pipeline = append(pipeline, postSteps...)

	return common.NewPipelineExecutor(pipeline...).Finally(info.interpolateOutputs()).Finally(info.closeContainer())
}
