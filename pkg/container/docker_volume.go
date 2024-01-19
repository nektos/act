//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/nektos/act/pkg/common"
)

func NewDockerVolumeRemoveExecutor(volumeName string, force bool) common.Executor {
	return func(ctx context.Context) error {
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
		defer cli.Close()

		list, err := cli.VolumeList(ctx, volume.ListOptions{Filters: filters.NewArgs()})
		if err != nil {
			return err
		}

		for _, vol := range list.Volumes {
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

		return cli.VolumeRemove(ctx, volume, force)
	}
}
