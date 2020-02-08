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

// StepEnv returns the env for a step
func (rc *RunContext) StepEnv(step *model.Step) map[string]string {
	env := make(map[string]string)
	env["HOME"] = "/github/home"
	env["GITHUB_WORKFLOW"] = rc.Run.Workflow.Name
	env["GITHUB_RUN_ID"] = "1"
	env["GITHUB_RUN_NUMBER"] = "1"
	env["GITHUB_ACTION"] = step.ID
	env["GITHUB_ACTOR"] = "nektos/act"

	repoPath := rc.Config.Workdir
	repo, err := common.FindGithubRepo(repoPath)
	if err != nil {
		log.Warningf("unable to get git repo: %v", err)
	} else {
		env["GITHUB_REPOSITORY"] = repo
	}
	env["GITHUB_EVENT_NAME"] = rc.Config.EventName
	env["GITHUB_EVENT_PATH"] = "/github/workflow/event.json"
	env["GITHUB_WORKSPACE"] = "/github/workspace"

	_, rev, err := common.FindGitRevision(repoPath)
	if err != nil {
		log.Warningf("unable to get git revision: %v", err)
	} else {
		env["GITHUB_SHA"] = rev
	}

	ref, err := common.FindGitRef(repoPath)
	if err != nil {
		log.Warningf("unable to get git ref: %v", err)
	} else {
		log.Infof("using github ref: %s", ref)
		env["GITHUB_REF"] = ref
	}
	job := rc.Run.Job()
	if job.Container != nil {
		return mergeMaps(rc.GetEnv(), job.Container.Env, step.GetEnv(), env)
	}
	return mergeMaps(rc.GetEnv(), step.GetEnv(), env)
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
		containerSpec := new(model.ContainerSpec)

		var stepExecutor common.Executor
		if step.Run != "" {
			stepExecutor = common.NewPipelineExecutor(
				rc.setupContainerSpec(step, containerSpec),
				rc.pullImage(containerSpec),
				rc.runContainer(containerSpec),
			)
		} else if step.Uses != "" {
			stepExecutor = common.NewErrorExecutor(fmt.Errorf("Not yet implemented - job:%s step:%+v", rc.Run, step))
			// clone action repo
			// read action.yaml
			// if runs.using == node12, start node12 container and run `main`
			// if runs.using == docker, pull `image` and run
			// caputre output/commands
		} else {
			stepExecutor = common.NewErrorExecutor(fmt.Errorf("Unable to determine how to run job:%s step:%+v", rc.Run, step))
		}
		steps = append(steps, stepExecutor)
	}
	return common.NewPipelineExecutor(steps...).Finally(rc.Close)
}

func (rc *RunContext) setupContainerSpec(step *model.Step, containerSpec *model.ContainerSpec) common.Executor {
	return func(ctx context.Context) error {
		job := rc.Run.Job()

		containerSpec.Env = rc.StepEnv(step)

		if step.Uses != "" {
			containerSpec.Image = step.Uses
		} else if job.Container != nil {
			containerSpec.Image = job.Container.Image
			containerSpec.Args = rc.shellCommand(step.Shell, step.Run)
			containerSpec.Ports = job.Container.Ports
			containerSpec.Volumes = job.Container.Volumes
			containerSpec.Options = job.Container.Options
		} else if step.Run != "" {
			containerSpec.Image = platformImage(job.RunsOn)
			containerSpec.Args = rc.shellCommand(step.Shell, step.Run)
		} else {
			return fmt.Errorf("Unable to setup container for %s", step)
		}
		return nil
	}
}

func (rc *RunContext) shellCommand(shell string, run string) string {
	shellCommand := ""

	switch shell {
	case "", "bash":
		shellCommand = "bash --noprofile --norc -eo pipefail {0}"
	case "pwsh":
		shellCommand = "pwsh -command \"& '{0}'\""
	case "python":
		shellCommand = "python {0}"
	case "sh":
		shellCommand = "sh -e -c {0}"
	case "cmd":
		shellCommand = "%ComSpec% /D /E:ON /V:OFF /S /C \"CALL \"{0}\"\""
	case "powershell":
		shellCommand = "powershell -command \"& '{0}'\""
	default:
		shellCommand = shell
	}

	tempScript, err := ioutil.TempFile(rc.Tempdir, ".temp-script-")
	if err != nil {
		log.Fatalf("Unable to create temp script %v", err)
	}

	if _, err := tempScript.Write([]byte(run)); err != nil {
		log.Fatal(err)
	}
	log.Debugf("Wrote command '%s' to '%s'", run, tempScript.Name())
	if err := tempScript.Close(); err != nil {
		log.Fatal(err)
	}
	containerPath := fmt.Sprintf("/github/home/%s", filepath.Base(tempScript.Name()))
	cmd := strings.Replace(shellCommand, "{0}", containerPath, 1)
	log.Debugf("about to run %s", cmd)
	return cmd
}

func platformImage(platform string) string {
	switch platform {
	case "ubuntu-latest", "ubuntu-18.04":
		return "ubuntu:18.04"
	case "ubuntu-16.04":
		return "ubuntu:16.04"
	case "windows-latest", "windows-2019", "macos-latest", "macos-10.15":
		return ""
	default:
		return ""
	}
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
