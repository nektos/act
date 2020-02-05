package container

import (
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/nektos/act/pkg/common"
	log "github.com/sirupsen/logrus"
)

// NewDockerBuildExecutorInput the input for the NewDockerBuildExecutor function
type NewDockerBuildExecutorInput struct {
	DockerExecutorInput
	ContextDir string
	ImageTag   string
}

// NewDockerBuildExecutor function to create a run executor for the container
func NewDockerBuildExecutor(input NewDockerBuildExecutorInput) common.Executor {
	return func() error {
		input.Logger.Infof("docker build -t %s %s", input.ImageTag, input.ContextDir)
		if input.Dryrun {
			return nil
		}

		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return err
		}
		cli.NegotiateAPIVersion(input.Ctx)

		input.Logger.Debugf("Building image from '%v'", input.ContextDir)

		tags := []string{input.ImageTag}
		options := types.ImageBuildOptions{
			Tags: tags,
		}

		buildContext, err := createBuildContext(input.ContextDir, "Dockerfile")
		if err != nil {
			return err
		}

		defer buildContext.Close()

		input.Logger.Debugf("Creating image from context dir '%s' with tag '%s'", input.ContextDir, input.ImageTag)
		resp, err := cli.ImageBuild(input.Ctx, buildContext, options)

		err = input.logDockerResponse(resp.Body, err != nil)
		if err != nil {
			return err
		}
		return nil
	}

}
func createBuildContext(contextDir string, relDockerfile string) (io.ReadCloser, error) {
	log.Debugf("Creating archive for build context dir '%s' with relative dockerfile '%s'", contextDir, relDockerfile)

	// And canonicalize dockerfile name to a platform-independent one
	relDockerfile = archive.CanonicalTarNameForPath(relDockerfile)

	f, err := os.Open(filepath.Join(contextDir, ".dockerignore"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	defer f.Close()

	var excludes []string
	if err == nil {
		excludes, err = dockerignore.ReadAll(f)
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
	keepThem1, _ := fileutils.Matches(".dockerignore", excludes)
	keepThem2, _ := fileutils.Matches(relDockerfile, excludes)
	if keepThem1 || keepThem2 {
		includes = append(includes, ".dockerignore", relDockerfile)
	}

	compression := archive.Uncompressed
	buildCtx, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		Compression:     compression,
		ExcludePatterns: excludes,
		IncludeFiles:    includes,
	})
	if err != nil {
		return nil, err
	}

	return buildCtx, nil
}
