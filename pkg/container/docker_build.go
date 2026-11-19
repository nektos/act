//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
	"github.com/moby/moby/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"

	"github.com/nektos/act/pkg/common"
)

// NewDockerBuildExecutor function to create a run executor for the container
func NewDockerBuildExecutor(input NewDockerBuildExecutorInput) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		if input.Platform != "" {
			logger.Infof("%sdocker build -t %s --platform %s %s", logPrefix, input.ImageTag, input.Platform, input.ContextDir)
		} else {
			logger.Infof("%sdocker build -t %s %s", logPrefix, input.ImageTag, input.ContextDir)
		}
		if common.Dryrun(ctx) {
			return nil
		}

		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
		defer cli.Close()

		logger.Debugf("Building image from '%v'", input.ContextDir)

		// A streamed context can only be uploaded once. Keep it so a missing
		// content blob can be retried with --pull.
		var streamed []byte
		if input.BuildContext != nil {
			streamed, err = io.ReadAll(input.BuildContext)
			if err != nil {
				return err
			}
		}

		options := imageBuildOptions(ctx, input)
		err = imageBuild(ctx, cli, options, buildContextReader(ctx, input, streamed))
		if err == nil || !isMissingImageContent(err) {
			return err
		}

		// The containerd image store can keep a multi-platform index after a
		// foreign-arch pull whose host-platform blob was never downloaded.
		// A later build of that name fails with "content digest ... not found"
		// instead of pulling the missing platform. Drop the tag and pull parents.
		logger.Infof("%sdocker build --pull -t %s %s", logPrefix, input.ImageTag, input.ContextDir)
		if _, rmErr := cli.ImageRemove(ctx, input.ImageTag, client.ImageRemoveOptions{
			Force:         true,
			PruneChildren: true,
		}); rmErr != nil && !cerrdefs.IsNotFound(rmErr) {
			logger.Debugf("remove image %s before rebuild: %v", input.ImageTag, rmErr)
		}
		options.PullParent = true
		return imageBuild(ctx, cli, options, buildContextReader(ctx, input, streamed))
	}
}

func imageBuildOptions(ctx context.Context, input NewDockerBuildExecutorInput) client.ImageBuildOptions {
	options := client.ImageBuildOptions{
		Tags:        []string{input.ImageTag},
		Remove:      true,
		AuthConfigs: LoadDockerAuthConfigs(ctx),
		Dockerfile:  input.Dockerfile,
	}
	if spec, ok := platformSpec(input.Platform); ok {
		options.Platforms = []specs.Platform{*spec}
	}
	return options
}

func buildContextReader(ctx context.Context, input NewDockerBuildExecutorInput, streamed []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		if input.BuildContext != nil {
			return io.NopCloser(bytes.NewReader(streamed)), nil
		}
		return createBuildContext(ctx, input.ContextDir, input.Dockerfile)
	}
}

func imageBuild(ctx context.Context, cli client.APIClient, options client.ImageBuildOptions, open func() (io.ReadCloser, error)) error {
	body, err := open()
	if err != nil {
		return err
	}
	defer body.Close()

	resp, err := cli.ImageBuild(ctx, body, options)
	if resp.Body == nil {
		return err
	}
	streamErr := logDockerResponse(common.Logger(ctx), resp.Body, err != nil)
	if streamErr != nil {
		return streamErr
	}
	return err
}

// isMissingImageContent reports the containerd image store error for a
// platform blob named by an index but not present locally.
func isMissingImageContent(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "content digest") && strings.Contains(msg, "not found")
}
func createBuildContext(ctx context.Context, contextDir string, relDockerfile string) (io.ReadCloser, error) {
	common.Logger(ctx).Debugf("Creating archive for build context dir '%s' with relative dockerfile '%s'", contextDir, relDockerfile)

	// And canonicalize dockerfile name to a platform-independent one
	relDockerfile = filepath.ToSlash(relDockerfile)

	f, err := os.Open(filepath.Join(contextDir, ".dockerignore"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	defer f.Close()

	var excludes []string
	if err == nil {
		excludes, err = ignorefile.ReadAll(f)
		if err != nil {
			return nil, err
		}
	}

	// If .dockerignore mentions .dockerignore or the Dockerfile
	// then make sure we send both files over to the daemon
	// because Dockerfile is, obviously, needed no matter what, and
	// .dockerignore is needed to know if either one needs to be
	// removed. The daemon will remove them for us, if needed, after it
	// parses the Dockerfile. Ignore errors here, as they will have been
	// caught by validateContextDirectory above.
	var includes = []string{"."}
	keepThem1, _ := patternmatcher.Matches(".dockerignore", excludes)
	keepThem2, _ := patternmatcher.Matches(relDockerfile, excludes)
	if keepThem1 || keepThem2 {
		includes = append(includes, ".dockerignore", relDockerfile)
	}

	buildCtx, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		Compression:     compression.None,
		ExcludePatterns: excludes,
		IncludeFiles:    includes,
	})
	if err != nil {
		return nil, err
	}

	return buildCtx, nil
}
