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
	err := rc.setupTempDir()
	if err != nil {
		return common.NewErrorExecutor(err)
	}
	steps := make([]common.Executor, 0)

	for i, step := range rc.Run.Job().Steps {
		if step.ID == "" {
			step.ID = fmt.Sprintf("%d", i)
		}
		s := step
		steps = append(steps, func(ctx context.Context) error {
			common.Logger(ctx).Infof("\u2B50  Run %s", s)
			err := rc.newStepExecutor(s)(ctx)
			if err == nil {
				common.Logger(ctx).Infof("  \u2705  Success - %s", s)
			} else {
				common.Logger(ctx).Errorf("  \u274C  Failure - %s", s)
			}
			return err
		})
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

func (rc *RunContext) setupTempDir() error {
	var err error
	tempBase := ""
	if runtime.GOOS == "darwin" {
		tempBase = "/tmp"
	}
	rc.Tempdir, err = ioutil.TempDir(tempBase, "act-")
	if err != nil {
		return err
	}
	err = os.Chmod(rc.Tempdir, 0755)
	if err != nil {
		return err
	}
	log.Debugf("Setup tempdir %s", rc.Tempdir)
	return err
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

		rawLogger := common.Logger(ctx).WithField("raw_output", true)
		logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) {
			rawLogger.Debugf(s)
		})

		return container.NewDockerRunExecutor(container.NewDockerRunExecutorInput{
			Cmd:        cmd,
			Entrypoint: entrypoint,
			Image:      containerSpec.Image,
			WorkingDir: "/github/workspace",
			Env:        envList,
			Name:       containerSpec.Name,
			Binds: []string{
				fmt.Sprintf("%s:%s", rc.Config.Workdir, "/github/workspace"),
				fmt.Sprintf("%s:%s", rc.Tempdir, "/github/home"),
				fmt.Sprintf("%s:%s", "/var/run/docker.sock", "/var/run/docker.sock"),
			},
			Content:         map[string]io.Reader{"/github": ghReader},
			ReuseContainers: rc.Config.ReuseContainers,
			Stdout:          logWriter,
			Stderr:          logWriter,
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

func (rc *RunContext) createContainerName(stepID string) string {
	containerName := fmt.Sprintf("%s-%s", stepID, rc.Tempdir)
	containerName = regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(containerName, "-")

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
