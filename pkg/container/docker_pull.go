package container

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/nektos/act/pkg/common"
)

// NewDockerPullExecutorInput the input for the NewDockerPullExecutor function
type NewDockerPullExecutorInput struct {
	Image     string
	ForcePull bool
	Platform  string
	Username  string
	Password  string
}

// NewDockerPullExecutor function to create a run executor for the container
func NewDockerPullExecutor(input NewDockerPullExecutorInput) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		logger.Debugf("%sdocker pull %v", logPrefix, input.Image)

		if common.Dryrun(ctx) {
			return nil
		}

		pull := input.ForcePull
		if !pull {
			imageExists, err := ImageExistsLocally(ctx, input.Image, input.Platform)
			log.Debugf("Image exists? %v", imageExists)
			if err != nil {
				return errors.WithMessagef(err, "unable to determine if image already exists for image %q (%s)", input.Image, input.Platform)
			}

			if !imageExists {
				pull = true
			}
		}

		if !pull {
			return nil
		}

		imageRef := cleanImage(input.Image)
		logger.Debugf("pulling image '%v' (%s)", imageRef, input.Platform)

		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}

		imagePullOptions, err := getImagePullOptions(ctx, input)
		if err != nil {
			return err
		}

		reader, err := cli.ImagePull(ctx, imageRef, imagePullOptions)

		_ = logDockerResponse(logger, reader, err != nil)
		if err != nil {
			return err
		}
		return nil
	}
}

func getImagePullOptions(ctx context.Context, input NewDockerPullExecutorInput) (types.ImagePullOptions, error) {
	imagePullOptions := types.ImagePullOptions{
		Platform: input.Platform,
	}
	if input.Username != "" && input.Password != "" {
		logger := common.Logger(ctx)
		logger.Debugf("using authentication for docker pull")

		authConfig := types.AuthConfig{
			Username: input.Username,
			Password: input.Password,
		}

		encodedJSON, err := json.Marshal(authConfig)
		if err != nil {
			return imagePullOptions, err
		}

		imagePullOptions.RegistryAuth = base64.URLEncoding.EncodeToString(encodedJSON)
	}

	return imagePullOptions, nil
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
