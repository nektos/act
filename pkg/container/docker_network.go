package container

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/nektos/act/pkg/common"
)

func NewDockerNetworkCreateExecutor(name string, config types.NetworkCreate) common.Executor {
	return func(ctx context.Context) error {
		if common.Dryrun(ctx) {
			return nil
		}

		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}

		if exists := DockerNetworkExists(ctx, name); exists {
			return nil
		}

		if _, err = cli.NetworkCreate(ctx, name, config); err != nil {
			return err
		}

		return nil
	}
}

func NewDockerNetworkRemoveExecutor(name string) common.Executor {
	return func(ctx context.Context) error {
		if common.Dryrun(ctx) {
			return nil
		}

		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}

		if err = cli.NetworkRemove(ctx, name); err != nil {
			return err
		}

		return nil
	}
}

func DockerNetworkExists(ctx context.Context, name string) bool {
	if _, exists, _ := GetDockerNetwork(ctx, name); !exists {
		return false
	}
	return true
}

func GetDockerNetwork(ctx context.Context, name string) (types.NetworkResource, bool, error) {
	cli, err := GetDockerClient(ctx)
	if err != nil {
		return types.NetworkResource{}, false, err
	}

	res, err := cli.NetworkInspect(ctx, name, types.NetworkInspectOptions{})
	if err != nil {
		return types.NetworkResource{}, false, err
	}

	return res, true, nil
}
