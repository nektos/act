package container

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/nektos/act/common"
	"golang.org/x/crypto/ssh/terminal"
)

// NewDockerRunExecutorInput the input for the NewDockerRunExecutor function
type NewDockerRunExecutorInput struct {
	DockerExecutorInput
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
	return func() error {

		input.Logger.Infof("docker run image=%s entrypoint=%+q cmd=%+q", input.Image, input.Entrypoint, input.Cmd)
		if input.Dryrun {
			return nil
		}

		var helper *connhelper.ConnectionHelper
		if host := os.Getenv("DOCKER_HOST"); host != "" {
			var err error
			helper, err = connhelper.GetConnectionHelper(host)
			if err != nil {
				return err
			}
		}
		cli, err := client.NewClientWithOpts(
				//client.FromEnv,
				client.WithHost(helper.Host),
				client.WithDialContext(helper.Dialer),
				)
		if err != nil {
			return err
		}
		cli.NegotiateAPIVersion(input.Ctx)

		// check if container exists
		containerID, err := findContainer(input, cli, input.Name)
		if err != nil {
			return err
		}

		// if we have an old container and we aren't reusing, remove it!
		if !input.ReuseContainers && containerID != "" {
			input.Logger.Debugf("Found existing container for %s...removing", input.Name)
			removeContainer(input, cli, containerID)
			containerID = ""
		}

		// create a new container if we don't have one to reuse
		if containerID == "" {
			containerID, err = createContainer(input, cli)
			if err != nil {
				return err
			}
		}

		// be sure to cleanup container if we aren't reusing
		if !input.ReuseContainers {
			defer removeContainer(input, cli, containerID)
		}

		executor := common.NewPipelineExecutor(
			func() error {
				return copyContentToContainer(input, cli, containerID)
			}, func() error {
				return attachContainer(input, cli, containerID)
			}, func() error {
				return startContainer(input, cli, containerID)
			}, func() error {
				return waitContainer(input, cli, containerID)
			},
		)
		return executor()
	}

}

func createContainer(input NewDockerRunExecutorInput, cli *client.Client) (string, error) {
	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))

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

	resp, err := cli.ContainerCreate(input.Ctx, config, &container.HostConfig{
		Binds: input.Binds,
	}, nil, input.Name)
	if err != nil {
		return "", err
	}
	input.Logger.Debugf("Created container name=%s id=%v from image %v", input.Name, resp.ID, input.Image)
	input.Logger.Debugf("ENV ==> %v", input.Env)

	return resp.ID, nil
}

func findContainer(input NewDockerRunExecutorInput, cli *client.Client, containerName string) (string, error) {
	containers, err := cli.ContainerList(input.Ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return "", err
	}

	for _, container := range containers {
		for _, name := range container.Names {
			if name[1:] == containerName {
				return container.ID, nil
			}
		}
	}

	return "", nil
}

func removeContainer(input NewDockerRunExecutorInput, cli *client.Client, containerID string) {
	err := cli.ContainerRemove(context.Background(), containerID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	if err != nil {
		input.Logger.Errorf("%v", err)
	}

	input.Logger.Debugf("Removed container: %v", containerID)
}

func copyContentToContainer(input NewDockerRunExecutorInput, cli *client.Client, containerID string) error {
	for dstPath, srcReader := range input.Content {
		input.Logger.Debugf("Extracting content to '%s'", dstPath)
		err := cli.CopyToContainer(input.Ctx, containerID, dstPath, srcReader, types.CopyToContainerOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func attachContainer(input NewDockerRunExecutorInput, cli *client.Client, containerID string) error {
	out, err := cli.ContainerAttach(input.Ctx, containerID, types.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return err
	}
	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
	if !isTerminal || os.Getenv("NORAW") != "" {
		go input.logDockerOutput(out.Reader)
	} else {
		go input.streamDockerOutput(out.Reader)
	}
	return nil
}

func startContainer(input NewDockerRunExecutorInput, cli *client.Client, containerID string) error {
	input.Logger.Debugf("STARTING image=%s entrypoint=%s cmd=%v", input.Image, input.Entrypoint, input.Cmd)

	if err := cli.ContainerStart(input.Ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	input.Logger.Debugf("Started container: %v", containerID)
	return nil
}

func waitContainer(input NewDockerRunExecutorInput, cli *client.Client, containerID string) error {
	statusCh, errCh := cli.ContainerWait(input.Ctx, containerID, container.WaitConditionNotRunning)
	var statusCode int64
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		statusCode = status.StatusCode
	}

	input.Logger.Debugf("Return status: %v", statusCode)

	if statusCode == 0 {
		return nil
	} else if statusCode == 78 {
		return fmt.Errorf("exiting with `NEUTRAL`: 78")
	}

	return fmt.Errorf("exit with `FAILURE`: %v", statusCode)
}
