package container

import (
	"context"

	"github.com/nektos/act/pkg/common"
)

// NewDockerVolumeRemoveExecutor function
func NewDockerVolumeRemoveExecutor(volume string, force bool) common.Executor {
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

		return cli.VolumeRemove(ctx, volume, force)
	}

}
