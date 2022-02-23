package runner

import (
	"context"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/nektos/act/pkg/common"
)

func (sc *StepContext) runUsesContainer() common.Executor {
	rc := sc.RunContext
	step := sc.Step
	return func(ctx context.Context) error {
		image := strings.TrimPrefix(step.Uses, "docker://")
		eval := sc.RunContext.NewExpressionEvaluator()
		cmd, err := shellquote.Split(eval.Interpolate(step.With["args"]))
		if err != nil {
			return err
		}
		entrypoint := strings.Fields(eval.Interpolate(step.With["entrypoint"]))
		stepContainer := sc.newStepContainer(ctx, image, cmd, entrypoint)

		return common.NewPipelineExecutor(
			stepContainer.Pull(rc.Config.ForcePull),
			stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
			stepContainer.Create(rc.Config.ContainerCapAdd, rc.Config.ContainerCapDrop),
			stepContainer.Start(true),
		).Finally(
			stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
		).Finally(stepContainer.Close())(ctx)
	}
}
