package runner

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

func (rc *RunContext) newStepExecutor(step *model.Step) common.Executor {
	job := rc.Run.Job()
	containerSpec := new(model.ContainerSpec)
	containerSpec.Env = rc.StepEnv(step)

	switch step.Type() {
	case model.StepTypeRun:
		if job.Container != nil {
			containerSpec.Image = job.Container.Image
			containerSpec.Ports = job.Container.Ports
			containerSpec.Volumes = job.Container.Volumes
			containerSpec.Options = job.Container.Options
		} else {
			containerSpec.Image = platformImage(job.RunsOn)
		}
		return common.NewPipelineExecutor(
			rc.setupShellCommand(containerSpec, step.Shell, step.Run),
			rc.pullImage(containerSpec),
			rc.runContainer(containerSpec),
		)

	case model.StepTypeUsesDockerURL:
		containerSpec.Image = strings.TrimPrefix(step.Uses, "docker://")
		containerSpec.Entrypoint = step.With["entrypoint"]
		containerSpec.Args = step.With["args"]
		return common.NewPipelineExecutor(
			rc.pullImage(containerSpec),
			rc.runContainer(containerSpec),
		)

	case model.StepTypeUsesActionLocal:
		return common.NewPipelineExecutor(
			rc.setupAction(containerSpec, filepath.Join(rc.Config.Workdir, step.Uses)),
			rc.pullImage(containerSpec),
			rc.runContainer(containerSpec),
		)
	case model.StepTypeUsesActionRemote:
		return common.NewPipelineExecutor(
			rc.cloneAction(step.Uses),
			rc.setupAction(containerSpec, step.Uses),
			rc.pullImage(containerSpec),
			rc.runContainer(containerSpec),
		)
	}

	return common.NewErrorExecutor(fmt.Errorf("Unable to determine how to run job:%s step:%+v", rc.Run, step))
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

func (rc *RunContext) setupShellCommand(containerSpec *model.ContainerSpec, shell string, run string) common.Executor {
	return func(ctx context.Context) error {
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
			return err
		}

		if _, err := tempScript.Write([]byte(run)); err != nil {
			return err
		}
		log.Debugf("Wrote command '%s' to '%s'", run, tempScript.Name())
		if err := tempScript.Close(); err != nil {
			return err
		}
		containerPath := fmt.Sprintf("/github/home/%s", filepath.Base(tempScript.Name()))
		containerSpec.Args = strings.Replace(shellCommand, "{0}", containerPath, 1)
		return nil
	}
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

func (rc *RunContext) setupAction(containerSpec *model.ContainerSpec, actionDir string) common.Executor {
	return func(ctx context.Context) error {
		f, err := os.Open(filepath.Join(actionDir, "action.yml"))
		if os.IsNotExist(err) {
			f, err = os.Open(filepath.Join(actionDir, "action.yaml"))
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		action, err := model.ReadAction(f)
		if err != nil {
			return err
		}

		for inputID, input := range action.Inputs {
			envKey := fmt.Sprintf("INPUT_%s", strings.ToUpper(inputID))
			envKey = regexp.MustCompile("[^A-Z0-9]").ReplaceAllString(envKey, "_")
			if _, ok := containerSpec.Env[envKey]; !ok {
				containerSpec.Env[envKey] = input.Default
			}
		}

		switch action.Runs.Using {
		case model.ActionRunsUsingNode12:
			containerSpec.Image = "node:12"
			containerSpec.Args = action.Runs.Main
		case model.ActionRunsUsingDocker:
			if strings.HasPrefix(action.Runs.Image, "docker://") {
				containerSpec.Image = strings.TrimPrefix(action.Runs.Image, "docker://")
				containerSpec.Entrypoint = strings.Join(action.Runs.Entrypoint, " ")
				containerSpec.Args = strings.Join(action.Runs.Args, " ")
			} else {
				// TODO: docker build
			}
		}
		return nil
	}
}

func (rc *RunContext) cloneAction(action string) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}
