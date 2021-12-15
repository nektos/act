package container

import (
	"context"

	"github.com/nektos/act/pkg/common/dryrun"
	"github.com/nektos/act/pkg/common/executor"
	"github.com/nektos/act/pkg/common/logger"

	"github.com/docker/docker/api/types/filters"
)

func NewDockerVolumeRemoveExecutor(volume string, force bool) executor.Executor {
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

func removeExecutor(volume string, force bool) executor.Executor {
	return func(ctx context.Context) error {
		logger := logger.Logger(ctx)
		logger.WithField("emoji", logPrefix).Infof("  docker volume rm %s", volume)

		if dryrun.Dryrun(ctx) {
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
