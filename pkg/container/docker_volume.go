//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"

	"github.com/moby/moby/client"
	"github.com/nektos/act/pkg/common"
)

func NewDockerVolumeRemoveExecutor(volumeName string, force bool) common.Executor {
	return func(ctx context.Context) error {
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
		defer cli.Close()

		list, err := cli.VolumeList(ctx, client.VolumeListOptions{})
		if err != nil {
			return err
		}

		for _, vol := range list.Items {
			if vol.Name == volumeName {
				return removeExecutor(volumeName, force)(ctx)
			}
		}

		// Volume not found - do nothing
		return nil
	}
}

func removeExecutor(volume string, force bool) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		logger.Debugf("%sdocker volume rm %s", logPrefix, volume)

		if common.Dryrun(ctx) {
			return nil
		}

		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
		defer cli.Close()

		_, err = cli.VolumeRemove(ctx, volume, client.VolumeRemoveOptions{Force: force})
		return err
	}
}
