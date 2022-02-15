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
	newStepExecutor(step *model.Step) common.Executor
	interpolateOutputs() common.Executor
	result(result string)
}

func newJobExecutor(info jobInfo) common.Executor {
	steps := make([]common.Executor, 0)

	steps = append(steps, func(ctx context.Context) error {
		if len(info.matrix()) > 0 {
			common.Logger(ctx).Infof("\U0001F9EA  Matrix: %v", info.matrix())
		}
		return nil
	})

	steps = append(steps, info.startContainer())

	for i, step := range info.steps() {
		if step.ID == "" {
			step.ID = fmt.Sprintf("%d", i)
		}
		stepExec := info.newStepExecutor(step)
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
	}

	steps = append(steps, func(ctx context.Context) error {
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

	return common.NewPipelineExecutor(steps...).Finally(info.interpolateOutputs()).Finally(info.closeContainer())
}
