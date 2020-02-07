package container

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/nektos/act/pkg/common"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
)

// NewDockerRunExecutorInput the input for the NewDockerRunExecutor function
type NewDockerRunExecutorInput struct {
	Image           string
	Entrypoint      []string
	Cmd             []string
	WorkingDir      string
	Env             []string
	Binds           []string
	Content         map[string]io.Reader
	Volumes         []string
	Name            string
	ReuseContainers bool
}

// NewDockerRunExecutor function to create a run executor for the container
func NewDockerRunExecutor(input NewDockerRunExecutorInput) common.Executor {
	cr := new(containerReference)
	cr.input = input

	return common.
		NewInfoExecutor("docker run image=%s entrypoint=%+q cmd=%+q", input.Image, input.Entrypoint, input.Cmd).
		Then(
			common.NewPipelineExecutor(
				cr.connect(),
				cr.find(),
				cr.remove().IfBool(!input.ReuseContainers),
				cr.create(),
				cr.copyContent(),
				cr.attach(),
				cr.start(),
				cr.wait(),
			).Finally(
				cr.remove().IfBool(!input.ReuseContainers),
			).IfNot(common.Dryrun),
		)
}

type containerReference struct {
	input NewDockerRunExecutorInput
	cli   *client.Client
	id    string
}

func (cr *containerReference) connect() common.Executor {
	return func(ctx context.Context) error {
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return errors.WithStack(err)
		}
		cli.NegotiateAPIVersion(ctx)
		cr.cli = cli
		return nil
	}
}

func (cr *containerReference) find() common.Executor {
	return func(ctx context.Context) error {
		containers, err := cr.cli.ContainerList(ctx, types.ContainerListOptions{
			All: true,
		})
		if err != nil {
			return errors.WithStack(err)
		}

		for _, container := range containers {
			for _, name := range container.Names {
				if name[1:] == cr.input.Name {
					cr.id = container.ID
					return nil
				}
			}
		}

		cr.id = ""
		return nil
	}
}

func (cr *containerReference) remove() common.Executor {
	return func(ctx context.Context) error {
		if cr.id == "" {
			return nil
		}

		logger := common.Logger(ctx)
		err := cr.cli.ContainerRemove(context.Background(), cr.id, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
		if err != nil {
			return errors.WithStack(err)
		}
		cr.id = ""

		logger.Debugf("Removed container: %v", cr.id)
		return nil
	}
}

func (cr *containerReference) create() common.Executor {
	return func(ctx context.Context) error {
		if cr.id != "" {
			return nil
		}
		logger := common.Logger(ctx)
		isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))

		input := cr.input
		config := &container.Config{
			Image:      input.Image,
			Cmd:        input.Cmd,
			Entrypoint: input.Entrypoint,
			WorkingDir: input.WorkingDir,
			Env:        input.Env,
			Tty:        isTerminal,
		}

		if len(input.Volumes) > 0 {
			config.Volumes = make(map[string]struct{})
			for _, vol := range input.Volumes {
				config.Volumes[vol] = struct{}{}
			}
		}

		resp, err := cr.cli.ContainerCreate(ctx, config, &container.HostConfig{
			Binds: input.Binds,
		}, nil, input.Name)
		if err != nil {
			return errors.WithStack(err)
		}
		logger.Debugf("Created container name=%s id=%v from image %v", input.Name, resp.ID, input.Image)
		logger.Debugf("ENV ==> %v", input.Env)

		cr.id = resp.ID
		return nil
	}
}

func (cr *containerReference) copyContent() common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		for dstPath, srcReader := range cr.input.Content {
			logger.Debugf("Extracting content to '%s'", dstPath)
			err := cr.cli.CopyToContainer(ctx, cr.id, dstPath, srcReader, types.CopyToContainerOptions{})
			if err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	}
}

func (cr *containerReference) attach() common.Executor {
	return func(ctx context.Context) error {
		out, err := cr.cli.ContainerAttach(ctx, cr.id, types.ContainerAttachOptions{
			Stream: true,
			Stdout: true,
			Stderr: true,
		})
		if err != nil {
			return errors.WithStack(err)
		}
		isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
		if !isTerminal || os.Getenv("NORAW") != "" {
			go logDockerOutput(ctx, out.Reader)
		} else {
			go streamDockerOutput(ctx, out.Reader)
		}
		return nil
	}
}

func (cr *containerReference) start() common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		logger.Debugf("STARTING image=%s entrypoint=%s cmd=%v", cr.input.Image, cr.input.Entrypoint, cr.input.Cmd)

		if err := cr.cli.ContainerStart(ctx, cr.id, types.ContainerStartOptions{}); err != nil {
			return errors.WithStack(err)
		}

		logger.Debugf("Started container: %v", cr.id)
		return nil
	}
}

func (cr *containerReference) wait() common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		statusCh, errCh := cr.cli.ContainerWait(ctx, cr.id, container.WaitConditionNotRunning)
		var statusCode int64
		select {
		case err := <-errCh:
			if err != nil {
				return errors.WithStack(err)
			}
		case status := <-statusCh:
			statusCode = status.StatusCode
		}

		logger.Debugf("Return status: %v", statusCode)

		if statusCode == 0 {
			return nil
		} else if statusCode == 78 {
			return fmt.Errorf("exit with `NEUTRAL`: 78")
		}

		return fmt.Errorf("exit with `FAILURE`: %v", statusCode)
	}
}
