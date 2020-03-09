package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

// StepContext contains info about current job
type StepContext struct {
	RunContext *RunContext
	Step       *model.Step
	Env        map[string]string
	Cmd        []string
	Action     *model.Action
}

func (sc *StepContext) execJobContainer() common.Executor {
	return func(ctx context.Context) error {
		return sc.RunContext.execJobContainer(sc.Cmd, sc.Env)(ctx)
	}
}

// Executor for a step context
func (sc *StepContext) Executor() common.Executor {
	rc := sc.RunContext
	step := sc.Step

	switch step.Type() {
	case model.StepTypeRun:
		return common.NewPipelineExecutor(
			sc.setupShellCommand(),
			sc.execJobContainer(),
		)

	case model.StepTypeUsesDockerURL:
		return common.NewPipelineExecutor(
			sc.runUsesContainer(),
		)

	case model.StepTypeUsesActionLocal:
		actionDir := filepath.Join(rc.Config.Workdir, step.Uses)
		return common.NewPipelineExecutor(
			sc.setupAction(actionDir, ""),
			sc.runAction(actionDir, ""),
		)
	case model.StepTypeUsesActionRemote:
		remoteAction := newRemoteAction(step.Uses)
		if remoteAction.IsCheckout() && rc.getGithubContext().isLocalCheckout(step) {
			return func(ctx context.Context) error {
				common.Logger(ctx).Debugf("Skipping actions/checkout")
				return nil
			}
		}

		actionDir := fmt.Sprintf("%s/%s", rc.ActionCacheDir(), strings.ReplaceAll(step.Uses, "/", "-"))
		return common.NewPipelineExecutor(
			common.NewGitCloneExecutor(common.NewGitCloneExecutorInput{
				URL: remoteAction.CloneURL(),
				Ref: remoteAction.Ref,
				Dir: actionDir,
			}),
			sc.setupAction(actionDir, remoteAction.Path),
			sc.runAction(actionDir, remoteAction.Path),
		)
	}

	return common.NewErrorExecutor(fmt.Errorf("Unable to determine how to run job:%s step:%+v", rc.Run, step))
}

func (sc *StepContext) setupEnv() common.Executor {
	rc := sc.RunContext
	job := rc.Run.Job()
	step := sc.Step
	return func(ctx context.Context) error {
		var env map[string]string
		c := job.Container()
		if c != nil {
			env = mergeMaps(rc.GetEnv(), c.Env, step.GetEnv())
		} else {
			env = mergeMaps(rc.GetEnv(), step.GetEnv())
		}

		for k, v := range env {
			env[k] = rc.ExprEval.Interpolate(v)
		}
		sc.Env = rc.withGithubEnv(env)
		log.Debugf("setupEnv => %v", sc.Env)
		return nil
	}
}

func (sc *StepContext) setupShellCommand() common.Executor {
	rc := sc.RunContext
	step := sc.Step
	return func(ctx context.Context) error {
		var script strings.Builder

		_, err := script.WriteString(fmt.Sprintf("PATH=\"%s:${PATH}\"\n", strings.Join(rc.ExtraPath, ":")))
		if err != nil {
			return err
		}

		run := rc.ExprEval.Interpolate(step.Run)

		if _, err = script.WriteString(run); err != nil {
			return err
		}
		scriptName := fmt.Sprintf("workflow/%s", step.ID)
		log.Debugf("Wrote command '%s' to '%s'", run, scriptName)
		containerPath := fmt.Sprintf("/github/%s", scriptName)
		sc.Cmd = strings.Fields(strings.Replace(step.ShellCommand(), "{0}", containerPath, 1))
		return rc.JobContainer.Copy("/github/", &container.FileEntry{
			Name: scriptName,
			Mode: 755,
			Body: script.String(),
		})(ctx)
	}
}

func (sc *StepContext) newStepContainer(ctx context.Context, image string, cmd []string, entrypoint []string) container.Container {
	rc := sc.RunContext
	step := sc.Step
	rawLogger := common.Logger(ctx).WithField("raw_output", true)
	logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) bool {
		if rc.Config.LogOutput {
			rawLogger.Infof(s)
		} else {
			rawLogger.Debugf(s)
		}
		return true
	})
	envList := make([]string, 0)
	for k, v := range sc.Env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	stepEE := sc.NewExpressionEvaluator()
	for i, v := range cmd {
		cmd[i] = stepEE.Interpolate(v)
	}
	for i, v := range entrypoint {
		entrypoint[i] = stepEE.Interpolate(v)
	}

	bindModifiers := ""
	if runtime.GOOS == "darwin" {
		bindModifiers = ":delegated"
	}

	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TOOL_CACHE", "/opt/hostedtoolcache"))

	binds := []string{
		fmt.Sprintf("%s:%s", "/var/run/docker.sock", "/var/run/docker.sock"),
	}
	if rc.Config.BindWorkdir {
		binds = append(binds, fmt.Sprintf("%s:%s%s", rc.Config.Workdir, "/github/workspace", bindModifiers))
	}

	stepContainer := container.NewContainer(&container.NewContainerInput{
		Cmd:        cmd,
		Entrypoint: entrypoint,
		WorkingDir: "/github/workspace",
		Image:      image,
		Name:       createContainerName(rc.jobContainerName(), step.ID),
		Env:        envList,
		Mounts: map[string]string{
			rc.jobContainerName(): "/github",
			"act-toolcache":       "/toolcache",
			"act-actions":         "/actions",
		},
		Binds:  binds,
		Stdout: logWriter,
		Stderr: logWriter,
	})
	return stepContainer
}
func (sc *StepContext) runUsesContainer() common.Executor {
	rc := sc.RunContext
	step := sc.Step
	return func(ctx context.Context) error {
		image := strings.TrimPrefix(step.Uses, "docker://")
		cmd := strings.Fields(step.With["args"])
		entrypoint := strings.Fields(step.With["entrypoint"])
		stepContainer := sc.newStepContainer(ctx, image, cmd, entrypoint)

		return common.NewPipelineExecutor(
			stepContainer.Pull(rc.Config.ForcePull),
			stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
			stepContainer.Create(),
			stepContainer.Start(true),
		).Finally(
			stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
		)(ctx)
	}
}

func (sc *StepContext) setupAction(actionDir string, actionPath string) common.Executor {
	return func(ctx context.Context) error {
		f, err := os.Open(filepath.Join(actionDir, actionPath, "action.yml"))
		if os.IsNotExist(err) {
			f, err = os.Open(filepath.Join(actionDir, actionPath, "action.yaml"))
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		sc.Action, err = model.ReadAction(f)
		log.Debugf("Read action %v from '%s'", sc.Action, f.Name())
		return err
	}
}

func (sc *StepContext) runAction(actionDir string, actionPath string) common.Executor {
	rc := sc.RunContext
	step := sc.Step
	return func(ctx context.Context) error {
		action := sc.Action
		log.Debugf("About to run action %v", action)
		for inputID, input := range action.Inputs {
			envKey := regexp.MustCompile("[^A-Z0-9-]").ReplaceAllString(strings.ToUpper(inputID), "_")
			envKey = fmt.Sprintf("INPUT_%s", envKey)
			if _, ok := sc.Env[envKey]; !ok {
				sc.Env[envKey] = rc.ExprEval.Interpolate(input.Default)
			}
		}

		actionName := ""
		containerActionDir := "."
		if step.Type() == model.StepTypeUsesActionLocal {
			actionName = strings.TrimPrefix(strings.TrimPrefix(actionDir, rc.Config.Workdir), string(filepath.Separator))
			containerActionDir = "/github/workspace"
		} else if step.Type() == model.StepTypeUsesActionRemote {
			actionName = strings.TrimPrefix(strings.TrimPrefix(actionDir, rc.ActionCacheDir()), string(filepath.Separator))
			containerActionDir = "/actions"
		}

		if actionName == "" {
			actionName = filepath.Base(actionDir)
		}

		log.Debugf("type=%v actionDir=%s Workdir=%s ActionCacheDir=%s actionName=%s containerActionDir=%s", step.Type(), actionDir, rc.Config.Workdir, rc.ActionCacheDir(), actionName, containerActionDir)

		switch action.Runs.Using {
		case model.ActionRunsUsingNode12:
			if step.Type() == model.StepTypeUsesActionRemote {
				err := rc.JobContainer.CopyDir(containerActionDir+string(filepath.Separator), actionDir)(ctx)
				if err != nil {
					return err
				}
			}
			return rc.execJobContainer([]string{"node", filepath.Join(containerActionDir, actionName, actionPath, action.Runs.Main)}, sc.Env)(ctx)
		case model.ActionRunsUsingDocker:
			var prepImage common.Executor
			var image string
			if strings.HasPrefix(action.Runs.Image, "docker://") {
				image = strings.TrimPrefix(action.Runs.Image, "docker://")
			} else {
				image = fmt.Sprintf("%s:%s", regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(actionName, "-"), "latest")
				image = fmt.Sprintf("act-%s", strings.TrimLeft(image, "-"))
				image = strings.ToLower(image)
				contextDir := filepath.Join(actionDir, actionPath, action.Runs.Main)
				prepImage = container.NewDockerBuildExecutor(container.NewDockerBuildExecutorInput{
					ContextDir: contextDir,
					ImageTag:   image,
				})
			}

			cmd := strings.Fields(step.With["args"])
			if len(cmd) == 0 {
				cmd = action.Runs.Args
			}
			entrypoint := strings.Fields(step.With["entrypoint"])
			if len(entrypoint) == 0 {
				entrypoint = action.Runs.Entrypoint
			}
			stepContainer := sc.newStepContainer(ctx, image, cmd, entrypoint)
			return common.NewPipelineExecutor(
				prepImage,
				stepContainer.Pull(rc.Config.ForcePull),
				stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
				stepContainer.Create(),
				stepContainer.Start(true),
			).Finally(
				stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
			)(ctx)
		}
		return nil
	}
}

type remoteAction struct {
	Org  string
	Repo string
	Path string
	Ref  string
}

func (ra *remoteAction) CloneURL() string {
	return fmt.Sprintf("https://github.com/%s/%s", ra.Org, ra.Repo)
}

func (ra *remoteAction) IsCheckout() bool {
	if ra.Org == "actions" && ra.Repo == "checkout" {
		return true
	}
	return false
}

func newRemoteAction(action string) *remoteAction {
	r := regexp.MustCompile(`^([^/@]+)/([^/@]+)(/([^@]*))?(@(.*))?$`)
	matches := r.FindStringSubmatch(action)

	ra := new(remoteAction)
	ra.Org = matches[1]
	ra.Repo = matches[2]
	ra.Path = ""
	ra.Ref = "master"
	if len(matches) >= 5 {
		ra.Path = matches[4]
	}
	if len(matches) >= 7 {
		ra.Ref = matches[6]
	}
	return ra
}
