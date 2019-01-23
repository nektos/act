package container

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/nektos/act/common"
)

// NewDockerPullExecutorInput the input for the NewDockerPullExecutor function
type NewDockerPullExecutorInput struct {
	DockerExecutorInput
	Image string
}

// NewDockerPullExecutor function to create a run executor for the container
func NewDockerPullExecutor(input NewDockerPullExecutorInput) common.Executor {
	return func() error {
		input.Logger.Infof("docker pull %v", input.Image)

		if input.Dryrun {
			return nil
		}

		imageRef := cleanImage(input.Image)
		input.Logger.Debugf("pulling image '%v'", imageRef)

		cli, err := getDockerClient(input.Ctx)
		if err != nil {
			return err
		}

		reader, err := cli.ImagePull(input.Ctx, imageRef, types.ImagePullOptions{})
		input.logDockerResponse(reader, err != nil)
		if err != nil {
			return err
		}
		return nil

	}

}

func cleanImage(image string) string {
	imageParts := len(strings.Split(image, "/"))
	if imageParts == 1 {
		image = fmt.Sprintf("docker.io/library/%s", image)
	} else if imageParts == 2 {
		image = fmt.Sprintf("docker.io/%s", image)
	}

	return image
}
