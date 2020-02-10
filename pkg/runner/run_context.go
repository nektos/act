package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/nektos/act/pkg/container"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

// RunContext contains info about current job
type RunContext struct {
	Config    *Config
	Run       *model.Run
	EventJSON string
	Env       map[string]string
	Outputs   map[string]string
	Tempdir   string
}

// GetEnv returns the env for the context
func (rc *RunContext) GetEnv() map[string]string {
	if rc.Env == nil {
		rc.Env = mergeMaps(rc.Run.Workflow.Env, rc.Run.Job().Env)
	}
	return rc.Env
}

// Close cleans up temp dir
func (rc *RunContext) Close(ctx context.Context) error {
	return os.RemoveAll(rc.Tempdir)
}

// Executor returns a pipeline executor for all the steps in the job
func (rc *RunContext) Executor() common.Executor {
	steps := make([]common.Executor, 0)
	steps = append(steps, rc.setupTempDir())

	for _, step := range rc.Run.Job().Steps {
		steps = append(steps, rc.newStepExecutor(step))
	}
	return common.NewPipelineExecutor(steps...).Finally(rc.Close)
}

func mergeMaps(maps ...map[string]string) map[string]string {
	rtnMap := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			rtnMap[k] = v
		}
	}
	return rtnMap
}

func (rc *RunContext) setupTempDir() common.Executor {
	return func(ctx context.Context) error {
		var err error
		tempBase := ""
		if runtime.GOOS == "darwin" {
			tempBase = "/tmp"
		}
		rc.Tempdir, err = ioutil.TempDir(tempBase, "act-")
		log.Debugf("Setup tempdir %s", rc.Tempdir)
		return err
	}
}

func (rc *RunContext) pullImage(containerSpec *model.ContainerSpec) common.Executor {
	return func(ctx context.Context) error {
		return container.NewDockerPullExecutor(container.NewDockerPullExecutorInput{
			Image:     containerSpec.Image,
			ForcePull: rc.Config.ForcePull,
		})(ctx)
	}
}

func (rc *RunContext) runContainer(containerSpec *model.ContainerSpec) common.Executor {
	return func(ctx context.Context) error {
		ghReader, err := rc.createGithubTarball()
		if err != nil {
			return err
		}

		envList := make([]string, 0)
		for k, v := range containerSpec.Env {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}
		var cmd, entrypoint []string
		if containerSpec.Args != "" {
			cmd = strings.Fields(containerSpec.Args)
		}
		if containerSpec.Entrypoint != "" {
			entrypoint = strings.Fields(containerSpec.Entrypoint)
		}

		return container.NewDockerRunExecutor(container.NewDockerRunExecutorInput{
			Cmd:        cmd,
			Entrypoint: entrypoint,
			Image:      containerSpec.Image,
			WorkingDir: "/github/workspace",
			Env:        envList,
			Name:       rc.createContainerName(),
			Binds: []string{
				fmt.Sprintf("%s:%s", rc.Config.Workdir, "/github/workspace"),
				fmt.Sprintf("%s:%s", rc.Tempdir, "/github/home"),
				fmt.Sprintf("%s:%s", "/var/run/docker.sock", "/var/run/docker.sock"),
			},
			Content:         map[string]io.Reader{"/github": ghReader},
			ReuseContainers: rc.Config.ReuseContainers,
		})(ctx)
	}
}

func (rc *RunContext) createGithubTarball() (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var files = []struct {
		Name string
		Mode int64
		Body string
	}{
		{"workflow/event.json", 0644, rc.EventJSON},
	}
	for _, file := range files {
		log.Debugf("Writing entry to tarball %s len:%d", file.Name, len(rc.EventJSON))
		hdr := &tar.Header{
			Name: file.Name,
			Mode: file.Mode,
			Size: int64(len(rc.EventJSON)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(rc.EventJSON)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil

}

func (rc *RunContext) createContainerName() string {
	containerName := regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(rc.Run.String(), "-")

	prefix := fmt.Sprintf("%s-", trimToLen(filepath.Base(rc.Config.Workdir), 10))
	suffix := ""
	containerName = trimToLen(containerName, 30-(len(prefix)+len(suffix)))
	return fmt.Sprintf("%s%s%s", prefix, containerName, suffix)
}

func trimToLen(s string, l int) string {
	if len(s) > l {
		return s[:l]
	}
	return s
}
