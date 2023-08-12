//go:build WITHOUT_DOCKER || !(linux || darwin || windows)

package container

import (
	"context"
	"runtime"

	"github.com/docker/docker/api/types"
	"github.com/nektos/act/pkg/common"
	"github.com/pkg/errors"
)

// ImageExistsLocally returns a boolean indicating if an image with the
// requested name, tag and architecture exists in the local docker image store
func ImageExistsLocally(ctx context.Context, imageName string, platform string) (bool, error) {
	return false, errors.New("Unsupported Operation")
}

// RemoveImage removes image from local store, the function is used to run different
// container image architectures
func RemoveImage(ctx context.Context, imageName string, force bool, pruneChildren bool) (bool, error) {
	return false, errors.New("Unsupported Operation")
}

// NewDockerBuildExecutor function to create a run executor for the container
func NewDockerBuildExecutor(input NewDockerBuildExecutorInput) common.Executor {
	return func(ctx context.Context) error {
		return errors.New("Unsupported Operation")
	}
}

// NewDockerPullExecutor function to create a run executor for the container
func NewDockerPullExecutor(input NewDockerPullExecutorInput) common.Executor {
	return func(ctx context.Context) error {
		return errors.New("Unsupported Operation")
	}
}

// NewContainer creates a reference to a container
func NewContainer(input *NewContainerInput) ExecutionsEnvironment {
	return nil
}

func RunnerArch(ctx context.Context) string {
	return runtime.GOOS
}

func GetHostInfo(ctx context.Context) (info types.Info, err error) {
	return types.Info{}, nil
}

func NewDockerVolumeRemoveExecutor(volume string, force bool) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func NewDockerNetworkCreateExecutor(name string) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func NewDockerNetworkRemoveExecutor(name string) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}
