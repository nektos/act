package container

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// ImageExistsLocally returns a boolean indicating if an image with the
// requested name (and tag) exist in the local docker image store
func ImageExistsLocally(ctx context.Context, imageName string) (bool, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return false, err
	}
	cli.NegotiateAPIVersion(ctx)

	filters := filters.NewArgs()
	filters.Add("reference", imageName)

	imageListOptions := types.ImageListOptions{
		Filters: filters,
	}

	images, err := cli.ImageList(ctx, imageListOptions)
	if err != nil {
		return false, err
	}

	return len(images) > 0, nil
}
