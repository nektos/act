package container

import (
	"context"

	"github.com/docker/docker/api/types/filters"
	"github.com/nektos/act/pkg/common"
)

func NewDockerVolumeRemoveExecutor(volume string, force bool) common.Executor {
	return func(ctx context.Context) error {
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
		defer cli.Close()

		list, err := cli.VolumeList(ctx, filters.NewArgs())
		if err != nil {
			return err
		}

		for _, vol := range list.Volumes {
			if vol.Name == volume {
				return removeExecutor(volume, force)(ctx)
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

		return cli.VolumeRemove(ctx, volume, force)
	}
}
