//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/nektos/act/pkg/common"
)

// ImageArchitectureMatches checks if the image architecture matches the requested platform
// and returns a boolean indicating if it does or not
func ImageArchitectureMatches(ctx context.Context, inspectImage image.InspectResponse, platform string) bool {
	imagePlatform := fmt.Sprintf("%s/%s", inspectImage.Os, inspectImage.Architecture)

	if imagePlatform == platform {
		return true
	}

	logger := common.Logger(ctx)
	logger.Infof("image found but platform does not match: %s (image) != %s (platform)\n", imagePlatform, platform)
	return false
}

// ImageExistsLocally returns a boolean indicating if an image with the
// requested name, tag and architecture exists in the local docker image store
func ImageExistsLocally(ctx context.Context, imageName string, platform string) (bool, error) {
	cli, err := GetDockerClient(ctx)
	if err != nil {
		return false, err
	}
	defer cli.Close()

	inspectImage, err := cli.ImageInspect(ctx, imageName)
	if client.IsErrNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if platform == "" || platform == "any" {
		return true, nil
	}

	return ImageArchitectureMatches(ctx, inspectImage, platform), nil
}

// RemoveImage removes image from local store, the function is used to run different
// container image architectures
func RemoveImage(ctx context.Context, imageName string, force bool, pruneChildren bool) (bool, error) {
	cli, err := GetDockerClient(ctx)
	if err != nil {
		return false, err
	}
	defer cli.Close()

	inspectImage, err := cli.ImageInspect(ctx, imageName)
	if client.IsErrNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if _, err = cli.ImageRemove(ctx, inspectImage.ID, image.RemoveOptions{
		Force:         force,
		PruneChildren: pruneChildren,
	}); err != nil {
		return false, err
	}

	return true, nil
}
