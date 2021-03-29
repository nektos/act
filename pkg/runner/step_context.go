package runner

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/kballard/go-shellquote"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
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

type formatError string

func (e formatError) Error() string {
	return fmt.Sprintf("Expected format {org}/{repo}[/path]@ref. Actual '%s' Input string was not in a correct format.", string(e))
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
		if remoteAction == nil {
			return common.NewErrorExecutor(formatError(step.Uses))
		}
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

func (sc *StepContext) mergeEnv() map[string]string {
	rc := sc.RunContext
	job := rc.Run.Job()
	step := sc.Step

	var env map[string]string
	c := job.Container()
	if c != nil {
		env = mergeMaps(rc.GetEnv(), c.Env, step.GetEnv())
	} else {
		env = mergeMaps(rc.GetEnv(), step.GetEnv())
	}

	if (rc.ExtraPath != nil) && (len(rc.ExtraPath) > 0) {
		s := append(rc.ExtraPath, `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`)
		env["PATH"] = strings.Join(s, `:`)
	}

	sc.Env = rc.withGithubEnv(env)
	return env
}

func (sc *StepContext) interpolateEnv(exprEval ExpressionEvaluator) {
	for k, v := range sc.Env {
		sc.Env[k] = exprEval.Interpolate(v)
	}
}

func (sc *StepContext) setupEnv(ctx context.Context) (ExpressionEvaluator, error) {
	rc := sc.RunContext
	sc.Env = sc.mergeEnv()
	if sc.Env != nil {
		err := rc.JobContainer.UpdateFromGithubEnv(&sc.Env)(ctx)
		if err != nil {
			return nil, err
		}
	}
	evaluator := sc.NewExpressionEvaluator()
	sc.interpolateEnv(evaluator)

	log.Debugf("setupEnv: %v", sc.Env)
	return evaluator, nil
}

func (sc *StepContext) setupShellCommand() common.Executor {
	rc := sc.RunContext
	step := sc.Step
	return func(ctx context.Context) error {
		var script strings.Builder
		var err error

		if step.WorkingDirectory == "" {
			step.WorkingDirectory = rc.Run.Job().Defaults.Run.WorkingDirectory
		}
		if step.WorkingDirectory == "" {
			step.WorkingDirectory = rc.Run.Workflow.Defaults.Run.WorkingDirectory
		}
		if step.WorkingDirectory != "" {
			_, err = script.WriteString(fmt.Sprintf("cd %s\n", step.WorkingDirectory))
			if err != nil {
				return err
			}
		}

		run := rc.ExprEval.Interpolate(step.Run)

		if _, err = script.WriteString(run); err != nil {
			return err
		}
		scriptName := fmt.Sprintf("workflow/%s", step.ID)
		log.Debugf("Wrote command '%s' to '%s'", run, scriptName)
		containerPath := fmt.Sprintf("/github/%s", scriptName)

		if step.Shell == "" {
			step.Shell = rc.Run.Job().Defaults.Run.Shell
		}
		if step.Shell == "" {
			step.Shell = rc.Run.Workflow.Defaults.Run.Shell
		}
		sc.Cmd = strings.Fields(strings.Replace(step.ShellCommand(), "{0}", containerPath, 1))
		return rc.JobContainer.Copy("/github/", &container.FileEntry{
			Name: scriptName,
			Mode: 0755,
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
			rawLogger.Infof("%s", s)
		} else {
			rawLogger.Debugf("%s", s)
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
	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_OS", "Linux"))
	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TEMP", "/tmp"))

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
		NetworkMode: fmt.Sprintf("container:%s", rc.jobContainerName()),
		Binds:       binds,
		Stdout:      logWriter,
		Stderr:      logWriter,
		Privileged:  rc.Config.Privileged,
		UsernsMode:  rc.Config.UsernsMode,
	})
	return stepContainer
}
func (sc *StepContext) runUsesContainer() common.Executor {
	rc := sc.RunContext
	step := sc.Step
	return func(ctx context.Context) error {
		image := strings.TrimPrefix(step.Uses, "docker://")
		cmd, err := shellquote.Split(sc.RunContext.NewExpressionEvaluator().Interpolate(step.With["args"]))
		if err != nil {
			return err
		}
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

func getOsSafeRelativePath(s, prefix string) string {
	actionName := strings.TrimPrefix(s, prefix)
	if runtime.GOOS == "windows" {
		actionName = strings.ReplaceAll(actionName, "\\", "/")
	}
	actionName = strings.TrimPrefix(actionName, "/")

	return actionName
}

func (sc *StepContext) getContainerActionPaths(step *model.Step, actionDir string, rc *RunContext) (string, string) {
	actionName := ""
	containerActionDir := "."
	if step.Type() == model.StepTypeUsesActionLocal {
		actionName = getOsSafeRelativePath(actionDir, rc.Config.Workdir)
		containerActionDir = "/github/workspace"
	} else if step.Type() == model.StepTypeUsesActionRemote {
		actionName = getOsSafeRelativePath(actionDir, rc.ActionCacheDir())
		containerActionDir = "/actions"
	}

	if actionName == "" {
		actionName = filepath.Base(actionDir)
		if runtime.GOOS == "windows" {
			actionName = strings.ReplaceAll(actionName, "\\", "/")
		}
	}
	return actionName, containerActionDir
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

		actionName, containerActionDir := sc.getContainerActionPaths(step, actionDir, rc)

		sc.Env = mergeMaps(sc.Env, action.Runs.Env)

		log.Debugf("type=%v actionDir=%s actionPath=%s Workdir=%s ActionCacheDir=%s actionName=%s containerActionDir=%s", step.Type(), actionDir, actionPath, rc.Config.Workdir, rc.ActionCacheDir(), actionName, containerActionDir)

		switch action.Runs.Using {
		case model.ActionRunsUsingNode12:
			if step.Type() == model.StepTypeUsesActionRemote {
				err := removeGitIgnore(actionDir)
				if err != nil {
					return err
				}
				err = rc.JobContainer.CopyDir(containerActionDir+"/", actionDir)(ctx)
				if err != nil {
					return err
				}
			}
			containerArgs := []string{"node", path.Join(containerActionDir, actionName, actionPath, action.Runs.Main)}
			log.Debugf("executing remote job container: %s", containerArgs)
			return rc.execJobContainer(containerArgs, sc.Env)(ctx)
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

			cmd, err := shellquote.Split(step.With["args"])
			if err != nil {
				return err
			}
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
		case model.ActionRunsUsingComposite:
			for outputName, output := range action.Outputs {
				re := regexp.MustCompile(`\${{ steps\.([a-zA-Z_][a-zA-Z0-9_-]+)\.outputs\.([a-zA-Z_][a-zA-Z0-9_-]+) }}`)
				matches := re.FindStringSubmatch(output.Value)
				if len(matches) > 2 {
					if sc.RunContext.OutputMappings == nil {
						sc.RunContext.OutputMappings = make(map[MappableOutput]MappableOutput)
					}

					k := MappableOutput{StepID: matches[1], OutputName: matches[2]}
					v := MappableOutput{StepID: step.ID, OutputName: outputName}
					sc.RunContext.OutputMappings[k] = v
				}
			}

			var executors []common.Executor
			stepID := 0
			for _, compositeStep := range action.Runs.Steps {
				stepClone := compositeStep
                // Take a copy of the run context structure (rc is a pointer)
                // Then take the address of the new structure
                rcCloneStr := *rc
                rcClone := &rcCloneStr
				if stepClone.ID == "" {
					stepClone.ID = fmt.Sprintf("composite-%d", stepID)
					stepID++
				}
                rcClone.CurrentStep = stepClone.ID

                if err := compositeStep.Validate(); err != nil {
                    return err
                }

                // Setup the outputs for the composite steps
                if _, ok := rcClone.StepResults[stepClone.ID]; ! ok {
                    rcClone.StepResults[stepClone.ID]  = &stepResult{
                        Success: true,
                        Outputs: make(map[string]string),
                    }
                }

				stepClone.Run = strings.ReplaceAll(stepClone.Run, "${{ github.action_path }}", filepath.Join(containerActionDir, actionName))

				stepContext := StepContext{
					RunContext: rcClone,
					Step:       &stepClone,
					Env:        mergeMaps(sc.Env, stepClone.Env),
				}

                // Interpolate the outer inputs into the composite step with items
                exprEval := sc.NewExpressionEvaluator()
                for k, v := range stepContext.Step.With {

                    if strings.Contains(v, "inputs") {
                        stepContext.Step.With[k] = exprEval.Interpolate(v)
                    }
                }

				executors = append(executors, stepContext.Executor())
			}
			return common.NewPipelineExecutor(executors...)(ctx)
		default:
			return fmt.Errorf(fmt.Sprintf("The runs.using key must be one of: %v, got %s", []string{
				model.ActionRunsUsingDocker,
				model.ActionRunsUsingNode12,
				model.ActionRunsUsingComposite,
			}, action.Runs.Using))
		}
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
	// GitHub's document[^] describes:
	// > We strongly recommend that you include the version of
	// > the action you are using by specifying a Git ref, SHA, or Docker tag number.
	// Actually, the workflow stops if there is the uses directive that hasn't @ref.
	// [^]: https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions
	r := regexp.MustCompile(`^([^/@]+)/([^/@]+)(/([^@]*))?(@(.*))?$`)
	matches := r.FindStringSubmatch(action)
	if len(matches) < 7 || matches[6] == "" {
		return nil
	}
	return &remoteAction{
		Org:  matches[1],
		Repo: matches[2],
		Path: matches[4],
		Ref:  matches[6],
	}
}

// https://github.com/nektos/act/issues/228#issuecomment-629709055
// files in .gitignore are not copied in a Docker container
// this causes issues with actions that ignore other important resources
// such as `node_modules` for example
func removeGitIgnore(directory string) error {
	gitIgnorePath := path.Join(directory, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); err == nil {
		// .gitignore exists
		log.Debugf("Removing %s before docker cp", gitIgnorePath)
		err := os.Remove(gitIgnorePath)
		if err != nil {
			return err
		}
	}
	return nil
}
