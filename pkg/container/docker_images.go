//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"
	"fmt"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/nektos/act/pkg/common"
)

// ImageExistsLocally returns a boolean indicating if an image with the
// requested name, tag and architecture exists in the local docker image store.
// A concrete platform is inspected with the daemon platform filter when the
// API supports it (1.49+).
func ImageExistsLocally(ctx context.Context, imageName string, platform string) (bool, error) {
	cli, err := GetDockerClient(ctx)
	if err != nil {
		return false, err
	}
	defer cli.Close()

	return imageExistsLocally(ctx, cli, imageName, platform)
}

func imageExistsLocally(ctx context.Context, cli client.APIClient, imageName string, platform string) (bool, error) {
	inspectImage, err := imageInspect(ctx, cli, imageName, platform)
	if cerrdefs.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	imagePlatform := fmt.Sprintf("%s/%s", inspectImage.Os, inspectImage.Architecture)
	if platform == "" || platform == "any" || imagePlatform == platform {
		return true, nil
	}

	common.Logger(ctx).Infof("image found but platform does not match: %s (image) != %s (platform)\n", imagePlatform, platform)
	return false, nil
}

// imageInspect loads image metadata. The containerd image store (Docker 29's
// default) answers an unscoped inspect with the host platform, and with an
// empty OS/architecture when the host platform is not present. A concrete
// platform is therefore passed through ImageInspectWithPlatform, which needs
// API 1.49. Older daemons reject that option; fall back to the unscoped
// inspect so Docker 28 / API 1.48 behavior is unchanged.
func imageInspect(ctx context.Context, cli client.APIClient, imageName, platform string) (client.ImageInspectResult, error) {
	spec, ok := platformSpec(platform)
	if !ok {
		return cli.ImageInspect(ctx, imageName)
	}

	inspected, err := cli.ImageInspect(ctx, imageName, client.ImageInspectWithPlatform(spec))
	if err == nil || !isUnsupportedImageInspectPlatform(err) {
		return inspected, err
	}
	return cli.ImageInspect(ctx, imageName)
}

// platformSpec parses the os/arch form used by pulls and builds. "" and "any"
// mean the caller does not want a platform filter.
func platformSpec(platform string) (*specs.Platform, bool) {
	if platform == "" || platform == "any" {
		return nil, false
	}
	parts := strings.SplitN(platform, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, false
	}
	return &specs.Platform{OS: parts[0], Architecture: parts[1]}, true
}

// isUnsupportedImageInspectPlatform reports the error moby's client returns
// from requiresVersion when ImageInspectWithPlatform is used against a daemon
// older than API 1.49. The feature name is quoted as `"platform"`.
func isUnsupportedImageInspectPlatform(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, `"platform"`) && strings.Contains(msg, "requires API version")
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
