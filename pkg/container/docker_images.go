package container

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// ImageExistsLocally returns a boolean indicating if an image with the
// requested name, tag and architecture exists in the local docker image store
func ImageExistsLocally(ctx context.Context, imageName string, platform string) (bool, string, error) {
	cli, err := GetDockerClient(ctx)
	if err != nil {
		return false, "", err
	}
	defer cli.Close()

	inspectImage, _, err := cli.ImageInspectWithRaw(ctx, imageName)
	if client.IsErrNotFound(err) {
		return false, "", nil
	} else if err != nil {
		return false, "", err
	}

	if platform == "" || platform == "any" || fmt.Sprintf("%s/%s", inspectImage.Os, inspectImage.Architecture) == platform {
		return true, inspectImage.ID, nil
	}

	return false, "", nil
}

// RemoveImage removes image from local store, the function is used to run different
// container image architectures
func RemoveImage(ctx context.Context, imageName string, force bool, pruneChildren bool) (bool, error) {
	exists, id, err := ImageExistsLocally(ctx, imageName, "any")
	if !exists {
		return false, err
	}

	cli, err := GetDockerClient(ctx)
	if err != nil {
		return false, err
	}
	defer cli.Close()

	if _, err = cli.ImageRemove(ctx, id, types.ImageRemoveOptions{
		Force:         force,
		PruneChildren: pruneChildren,
	}); err != nil {
		return false, err
	}

	return false, nil
}
