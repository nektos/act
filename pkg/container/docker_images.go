//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/nektos/act/pkg/common"
)

// ImageExistsLocally returns a boolean indicating if an image with the
// requested name, tag and architecture exists in the local docker image store
func ImageExistsLocally(ctx context.Context, imageName string, platform string) (bool, error) {
	cli, err := GetDockerClient(ctx)
	if err != nil {
		return false, err
	}
	defer cli.Close()

	return imageExistsLocally(ctx, cli, imageName, platform)
}

func imageExistsLocally(ctx context.Context, cli client.APIClient, imageName, platform string) (bool, error) {
	p := parsePlatform(platform)

	var opts []client.ImageInspectOption
	if p != nil && apiSupportsImageInspectPlatform(ctx, cli) {
		opts = append(opts, client.ImageInspectWithPlatform(p))
	}

	inspectImage, err := cli.ImageInspect(ctx, imageName, opts...)
	if cerrdefs.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	// No platform constraint, or the daemon already selected the requested
	// platform (API >= 1.49): existence of the inspect result is enough.
	if p == nil || len(opts) > 0 {
		return true, nil
	}

	if inspectImage.Os == p.OS && inspectImage.Architecture == p.Architecture &&
		(p.Variant == "" || inspectImage.Variant == p.Variant) {
		return true, nil
	}

	imagePlatform := fmt.Sprintf("%s/%s", inspectImage.Os, inspectImage.Architecture)
	logger := common.Logger(ctx)
	logger.Infof("image found but platform does not match: %s (image) != %s (platform)\n", imagePlatform, platform)

	return false, nil
}

// parsePlatform parses "os/arch" or "os/arch/variant". "" and "any" mean no
// platform constraint.
func parsePlatform(platform string) *specs.Platform {
	if platform == "" || platform == "any" {
		return nil
	}
	parts := strings.SplitN(platform, "/", 3)
	p := &specs.Platform{OS: parts[0]}
	if len(parts) > 1 {
		p.Architecture = parts[1]
	}
	if len(parts) > 2 {
		p.Variant = parts[2]
	}
	return p
}

// apiSupportsImageInspectPlatform reports whether the daemon accepts the
// platform option on image inspect (API 1.49 and later). With the containerd
// image store a tag can hold several platforms, and an inspect without a
// platform only describes the host variant.
func apiSupportsImageInspectPlatform(ctx context.Context, cli client.APIClient) bool {
	ver, err := cli.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		common.Logger(ctx).Debugf("failed to get Docker API version, not inspecting by platform: %v", err)
		return false
	}
	sv, err := semver.NewVersion(ver.APIVersion)
	if err != nil {
		common.Logger(ctx).Debugf("failed to parse Docker API version %q, not inspecting by platform: %v", ver.APIVersion, err)
		return false
	}
	constraint, _ := semver.NewConstraint(">= 1.49")
	return constraint.Check(sv)
}

// RemoveImage removes image from local store, the function is used to run different
// container image architectures
func RemoveImage(ctx context.Context, imageName string, force bool, pruneChildren bool) (bool, error) {
	cli, err := GetDockerClient(ctx)
	if err != nil {
		return false, err
	}
	defer cli.Close()

	inspectImage, err := cli.ImageInspect(ctx, imageName)
	if cerrdefs.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if _, err = cli.ImageRemove(ctx, inspectImage.ID, client.ImageRemoveOptions{
		Force:         force,
		PruneChildren: pruneChildren,
	}); err != nil {
		return false, err
	}

	return true, nil
}
