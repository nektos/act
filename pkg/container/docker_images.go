package container

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

// ImageExistsLocally returns a boolean indicating if an image with the
// requested name, tag and architecture exists in the local docker image store
func ImageExistsLocally(ctx context.Context, imageName string, platform string) (bool, error) {
	cli, err := GetDockerClient(ctx)
	if err != nil {
		return false, err
	}

	filters := filters.NewArgs()
	filters.Add("reference", imageName)

	imageListOptions := types.ImageListOptions{
		Filters: filters,
	}

	images, err := cli.ImageList(ctx, imageListOptions)
	if err != nil {
		return false, err
	}

	if len(images) > 0 {
		if platform == "any" {
			return true, nil
		}
		for _, v := range images {
			inspectImage, _, err := cli.ImageInspectWithRaw(ctx, v.ID)
			if err != nil {
				return false, err
			}

			if fmt.Sprintf("%s/%s", inspectImage.Os, inspectImage.Architecture) == platform {
				return true, nil
			}
		}
		return false, nil
	}

	return false, nil
}

// DeleteImage removes image from local store, the function is used to run different
// container image architectures
func DeleteImage(ctx context.Context, imageName string) (bool, error) {
	if exists, err := ImageExistsLocally(ctx, imageName, "any"); !exists {
		return false, err
	}

	cli, err := GetDockerClient(ctx)
	if err != nil {
		return false, err
	}

	filters := filters.NewArgs()
	filters.Add("reference", imageName)

	imageListOptions := types.ImageListOptions{
		Filters: filters,
	}

	images, err := cli.ImageList(ctx, imageListOptions)
	if err != nil {
		return false, err
	}

	if len(images) > 0 {
		for _, v := range images {
			if _, err = cli.ImageRemove(ctx, v.ID, types.ImageRemoveOptions{
				Force:         true,
				PruneChildren: true,
			}); err != nil {
				return false, err
			}
		}
		return true, nil
	}

	return false, nil
}
