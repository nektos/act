package container

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/nektos/act/pkg/common"
)

func NewDockerNetworkCreateExecutor(name string) common.Executor {
	return func(ctx context.Context) error {
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}

		network, err := cli.NetworkCreate(ctx, name, types.NetworkCreate{})
		if err != nil {
			return err
		}
		fmt.Printf("%#v", network.ID)

		return nil
	}
}

func NewDockerNetworkRemoveExecutor(name string) common.Executor {
	return func(ctx context.Context) error {
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}

		cli.NetworkRemove(ctx, name)
		return nil
	}
}
