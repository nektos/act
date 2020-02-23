package container

import (
	"context"

	"github.com/docker/docker/client"
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

		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return err
		}
		cli.NegotiateAPIVersion(ctx)

		return cli.VolumeRemove(ctx, volume, force)
	}

}
