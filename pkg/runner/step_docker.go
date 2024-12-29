package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
)

type stepDocker struct {
	Step       *model.Step
	RunContext *RunContext
	env        map[string]string
}

func (sd *stepDocker) pre() common.Executor {
	return func(_ context.Context) error {
		return nil
	}
}

func (sd *stepDocker) main() common.Executor {
	sd.env = map[string]string{}

	return runStepExecutor(sd, stepStageMain, sd.runUsesContainer())
}

func (sd *stepDocker) post() common.Executor {
	return func(_ context.Context) error {
		return nil
	}
}

func (sd *stepDocker) getRunContext() *RunContext {
	return sd.RunContext
}

func (sd *stepDocker) getGithubContext(ctx context.Context) *model.GithubContext {
	return sd.getRunContext().getGithubContext(ctx)
}

func (sd *stepDocker) getStepModel() *model.Step {
	return sd.Step
}

func (sd *stepDocker) getEnv() *map[string]string {
	return &sd.env
}

func (sd *stepDocker) getIfExpression(_ context.Context, _ stepStage) string {
	return sd.Step.If.Value
}

func (sd *stepDocker) runUsesContainer() common.Executor {
	rc := sd.RunContext
	step := sd.Step

	return func(ctx context.Context) error {
		image := strings.TrimPrefix(step.Uses, "docker://")
		eval := rc.NewExpressionEvaluator(ctx)
		cmd, err := shellquote.Split(eval.Interpolate(ctx, step.With["args"]))
		if err != nil {
			return err
		}

		var entrypoint []string
		if entry := eval.Interpolate(ctx, step.With["entrypoint"]); entry != "" {
			entrypoint = []string{entry}
		}

		stepContainer := sd.newStepContainer(ctx, image, cmd, entrypoint)

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

var (
	ContainerNewContainer = container.NewContainer
)

func (sd *stepDocker) newStepContainer(ctx context.Context, image string, cmd []string, entrypoint []string) container.Container {
	rc := sd.RunContext
	step := sd.Step

	rawLogger := common.Logger(ctx).WithField("raw_output", true)
	logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) bool {
		if rc.Config.LogOutput {
			rawLogger.Infof("%s", s)
		} else {
			rawLogger.Debugf("%s", s)
		}
		return true
	})
	envList := make([]string, 0)
	for k, v := range sd.env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TOOL_CACHE", "/opt/hostedtoolcache"))
	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_OS", "Linux"))
	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_ARCH", container.RunnerArch(ctx)))
	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TEMP", "/tmp"))

	binds, mounts := rc.GetBindsAndMounts()
	stepContainer := ContainerNewContainer(&container.NewContainerInput{
		Cmd:         cmd,
		Entrypoint:  entrypoint,
		WorkingDir:  rc.JobContainer.ToContainerPath(rc.Config.Workdir),
		Image:       image,
		Username:    rc.Config.Secrets["DOCKER_USERNAME"],
		Password:    rc.Config.Secrets["DOCKER_PASSWORD"],
		Name:        createContainerName(rc.jobContainerName(), step.ID),
		Env:         envList,
		Mounts:      mounts,
		NetworkMode: fmt.Sprintf("container:%s", rc.jobContainerName()),
		Binds:       binds,
		Stdout:      logWriter,
		Stderr:      logWriter,
		Privileged:  rc.Config.Privileged,
		UsernsMode:  rc.Config.UsernsMode,
		Platform:    rc.Config.ContainerArchitecture,
	})
	return stepContainer
}
