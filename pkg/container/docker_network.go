//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"

	"github.com/docker/docker/api/types/network"
	"github.com/nektos/act/pkg/common"
)

func NewDockerNetworkCreateExecutor(name string) common.Executor {
	return func(ctx context.Context) error {
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
		defer cli.Close()

		// Only create the network if it doesn't exist
		networks, err := cli.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return err
		}
		common.Logger(ctx).Debugf("%v", networks)
		for _, network := range networks {
			if network.Name == name {
				common.Logger(ctx).Debugf("Network %v exists", name)
				return nil
			}
		}

		_, err = cli.NetworkCreate(ctx, name, network.CreateOptions{
			Driver: "bridge",
			Scope:  "local",
		})
		if err != nil {
			return err
		}

		return nil
	}
}

func NewDockerNetworkRemoveExecutor(name string) common.Executor {
	return func(ctx context.Context) error {
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
		defer cli.Close()

		// Make sure that all network of the specified name are removed
		// cli.NetworkRemove refuses to remove a network if there are duplicates
		networks, err := cli.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return err
		}
		common.Logger(ctx).Debugf("%v", networks)
		for _, net := range networks {
			if net.Name == name {
				result, err := cli.NetworkInspect(ctx, net.ID, network.InspectOptions{})
				if err != nil {
					return err
				}

				if len(result.Containers) == 0 {
					if err = cli.NetworkRemove(ctx, net.ID); err != nil {
						common.Logger(ctx).Debugf("%v", err)
					}
				} else {
					common.Logger(ctx).Debugf("Refusing to remove network %v because it still has active endpoints", name)
				}
			}
		}

		return err
	}
}
