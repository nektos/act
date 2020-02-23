package runner

import (
	"context"
	"fmt"
	"regexp"
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
			sc.setupEnv(),
			sc.setupShellCommand(),
			sc.execJobContainer(),
		)

	case model.StepTypeUsesDockerURL:
		return common.NewPipelineExecutor(
			sc.setupEnv(),
			sc.runUsesContainer(),
		)

		/*
			case model.StepTypeUsesActionLocal:
				return common.NewPipelineExecutor(
					sc.setupEnv(),
					sc.setupAction(),
					applyWith(containerSpec, step),
					rc.pullImage(containerSpec),
					rc.runContainer(containerSpec),
				)
					case model.StepTypeUsesActionRemote:
						remoteAction := newRemoteAction(step.Uses)
						if remoteAction.Org == "actions" && remoteAction.Repo == "checkout" {
							return func(ctx context.Context) error {
								common.Logger(ctx).Debugf("Skipping actions/checkout")
								return nil
							}
						}
						cloneDir, err := ioutil.TempDir(rc.Tempdir, remoteAction.Repo)
						if err != nil {
							return common.NewErrorExecutor(err)
						}
						return common.NewPipelineExecutor(
							common.NewGitCloneExecutor(common.NewGitCloneExecutorInput{
								URL: remoteAction.CloneURL(),
								Ref: remoteAction.Ref,
								Dir: cloneDir,
							}),
							sc.setupEnv(),
							sc.setupAction(),
							applyWith(containerSpec, step),
							rc.pullImage(containerSpec),
							rc.runContainer(containerSpec),
						)
		*/
	}

	return common.NewErrorExecutor(fmt.Errorf("Unable to determine how to run job:%s step:%+v", rc.Run, step))
}

func applyWith(containerSpec *model.ContainerSpec, step *model.Step) common.Executor {
	return func(ctx context.Context) error {
		if entrypoint, ok := step.With["entrypoint"]; ok {
			containerSpec.Entrypoint = entrypoint
		}
		if args, ok := step.With["args"]; ok {
			containerSpec.Args = args
		}
		return nil
	}
}

func (sc *StepContext) setupEnv() common.Executor {
	rc := sc.RunContext
	job := rc.Run.Job()
	step := sc.Step
	return func(ctx context.Context) error {
		var env map[string]string
		if job.Container != nil {
			env = mergeMaps(rc.GetEnv(), job.Container.Env, step.GetEnv())
		} else {
			env = mergeMaps(rc.GetEnv(), step.GetEnv())
		}

		for k, v := range env {
			env[k] = rc.ExprEval.Interpolate(v)
		}
		sc.Env = rc.withGithubEnv(env)
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
	logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) {
		if rc.Config.LogOutput {
			rawLogger.Infof(s)
		} else {
			rawLogger.Debugf(s)
		}
	})
	envList := make([]string, 0)
	for k, v := range sc.Env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
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
		},
		Binds: []string{
			fmt.Sprintf("%s:%s", rc.Config.Workdir, "/github/workspace"),
			fmt.Sprintf("%s:%s", "/var/run/docker.sock", "/var/run/docker.sock"),
		},
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
		cmd := strings.Fields(rc.ExprEval.Interpolate(step.With["args"]))
		entrypoint := strings.Fields(rc.ExprEval.Interpolate(step.With["entrypoint"]))
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

/*

func (sc *StepContext) setupAction() common.Executor {
	rc := sc.RunContext
	step := sc.Step
	actionDir := filepath.Join(rc.Config.Workdir, step.Uses)
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
			envKey := regexp.MustCompile("[^A-Z0-9-]").ReplaceAllString(strings.ToUpper(inputID), "_")
			envKey = fmt.Sprintf("INPUT_%s", envKey)
			if _, ok := containerSpec.Env[envKey]; !ok {
				containerSpec.Env[envKey] = input.Default
			}
		}

		switch action.Runs.Using {
		case model.ActionRunsUsingNode12:
			if strings.HasPrefix(actionDir, rc.Config.Workdir) {
				containerSpec.Entrypoint = fmt.Sprintf("node /github/workspace/%s/%s", strings.TrimPrefix(actionDir, rc.Config.Workdir), action.Runs.Main)
			} else if strings.HasPrefix(actionDir, rc.Tempdir) {
				containerSpec.Entrypoint = fmt.Sprintf("node /github/home/%s/%s", strings.TrimPrefix(actionDir, rc.Tempdir), action.Runs.Main)
			}
		case model.ActionRunsUsingDocker:
			if strings.HasPrefix(actionDir, rc.Config.Workdir) {
				containerSpec.Name = rc.createStepContainerName(strings.TrimPrefix(actionDir, rc.Config.Workdir))
			} else if strings.HasPrefix(actionDir, rc.Tempdir) {
				containerSpec.Name = rc.createStepContainerName(strings.TrimPrefix(actionDir, rc.Tempdir))
			}
			containerSpec.Reuse = rc.Config.ReuseContainers
			if strings.HasPrefix(action.Runs.Image, "docker://") {
				containerSpec.Image = strings.TrimPrefix(action.Runs.Image, "docker://")
				containerSpec.Entrypoint = strings.Join(action.Runs.Entrypoint, " ")
				containerSpec.Args = strings.Join(action.Runs.Args, " ")
			} else {
				containerSpec.Image = fmt.Sprintf("%s:%s", containerSpec.Name, "latest")
				contextDir := filepath.Join(actionDir, action.Runs.Main)
				return container.NewDockerBuildExecutor(container.NewDockerBuildExecutorInput{
					ContextDir: contextDir,
					ImageTag:   containerSpec.Image,
				})(ctx)
			}
		}
		return nil
	}
}
*/

type remoteAction struct {
	Org  string
	Repo string
	Path string
	Ref  string
}

func (ra *remoteAction) CloneURL() string {
	return fmt.Sprintf("https://github.com/%s/%s", ra.Org, ra.Repo)
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
