package runner

import (
	"archive/tar"
	"context"
	"embed"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

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
	Needs      *model.Job
}

func (sc *StepContext) execJobContainer() common.Executor {
	return func(ctx context.Context) error {
		return sc.RunContext.execJobContainer(sc.Cmd, sc.Env, "", sc.Step.WorkingDirectory)(ctx)
	}
}

type formatError string

func (e formatError) Error() string {
	return fmt.Sprintf("Expected format {org}/{repo}[/path]@ref. Actual '%s' Input string was not in a correct format.", string(e))
}

// Executor for a step context
func (sc *StepContext) Executor(ctx context.Context) common.Executor {
	rc := sc.RunContext
	step := sc.Step

	switch step.Type() {
	case model.StepTypeRun:
		return common.NewPipelineExecutor(
			sc.setupShellCommandExecutor(),
			sc.execJobContainer(),
		)

	case model.StepTypeUsesDockerURL:
		return common.NewPipelineExecutor(
			sc.runUsesContainer(),
		)

	case model.StepTypeUsesActionLocal:
		actionDir := filepath.Join(rc.Config.Workdir, step.Uses)
		return common.NewPipelineExecutor(
			sc.setupAction(actionDir, "", true),
			sc.runAction(actionDir, "", "", "", true),
		)
	case model.StepTypeUsesActionRemote:
		remoteAction := newRemoteAction(step.Uses)
		if remoteAction == nil {
			return common.NewErrorExecutor(formatError(step.Uses))
		}

		remoteAction.URL = rc.Config.GitHubInstance

		github := rc.getGithubContext()
		if remoteAction.IsCheckout() && isLocalCheckout(github, step) {
			return func(ctx context.Context) error {
				common.Logger(ctx).Debugf("Skipping local actions/checkout because workdir was already copied")
				return nil
			}
		}

		actionDir := fmt.Sprintf("%s/%s", rc.ActionCacheDir(), strings.ReplaceAll(step.Uses, "/", "-"))
		gitClone := common.NewGitCloneExecutor(common.NewGitCloneExecutorInput{
			URL:   remoteAction.CloneURL(),
			Ref:   remoteAction.Ref,
			Dir:   actionDir,
			Token: github.Token,
		})
		var ntErr common.Executor
		if err := gitClone(ctx); err != nil {
			if err.Error() == "short SHA references are not supported" {
				err = errors.Cause(err)
				return common.NewErrorExecutor(fmt.Errorf("Unable to resolve action `%s`, the provided ref `%s` is the shortened version of a commit SHA, which is not supported. Please use the full commit SHA `%s` instead", step.Uses, remoteAction.Ref, err.Error()))
			} else if err.Error() != "some refs were not updated" {
				return common.NewErrorExecutor(err)
			} else {
				ntErr = common.NewInfoExecutor("Non-terminating error while running 'git clone': %v", err)
			}
		}
		return common.NewPipelineExecutor(
			ntErr,
			sc.setupAction(actionDir, remoteAction.Path, false),
			sc.runAction(actionDir, remoteAction.Path, remoteAction.Repo, remoteAction.Ref, false),
		)
	case model.StepTypeInvalid:
		return common.NewErrorExecutor(fmt.Errorf("Invalid run/uses syntax for job:%s step:%+v", rc.Run, step))
	}

	return common.NewErrorExecutor(fmt.Errorf("Unable to determine how to run job:%s step:%+v", rc.Run, step))
}

func (sc *StepContext) mergeEnv() map[string]string {
	rc := sc.RunContext
	job := rc.Run.Job()

	var env map[string]string
	c := job.Container()
	if c != nil {
		env = mergeMaps(rc.GetEnv(), c.Env)
	} else {
		env = rc.GetEnv()
	}

	if env["PATH"] == "" {
		env["PATH"] = `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`
	}
	if rc.ExtraPath != nil && len(rc.ExtraPath) > 0 {
		p := env["PATH"]
		env["PATH"] = strings.Join(rc.ExtraPath, `:`)
		env["PATH"] += `:` + p
	}

	sc.Env = rc.withGithubEnv(env)
	return env
}

func (sc *StepContext) interpolateEnv(exprEval ExpressionEvaluator) {
	for k, v := range sc.Env {
		sc.Env[k] = exprEval.Interpolate(v)
	}
}

func (sc *StepContext) isEnabled(ctx context.Context) (bool, error) {
	runStep, err := EvalBool(sc.NewExpressionEvaluator(), sc.Step.If.Value)
	if err != nil {
		common.Logger(ctx).Errorf("  \u274C  Error in if: expression - %s", sc.Step)
		exprEval, err := sc.setupEnv(ctx)
		if err != nil {
			return false, err
		}
		sc.RunContext.ExprEval = exprEval
		return false, err
	}

	return runStep, nil
}

func (sc *StepContext) setupEnv(ctx context.Context) (ExpressionEvaluator, error) {
	rc := sc.RunContext
	sc.Env = sc.mergeEnv()
	if sc.Env != nil {
		err := rc.JobContainer.UpdateFromImageEnv(&sc.Env)(ctx)
		if err != nil {
			return nil, err
		}
		err = rc.JobContainer.UpdateFromEnv(sc.Env["GITHUB_ENV"], &sc.Env)(ctx)
		if err != nil {
			return nil, err
		}
		err = rc.JobContainer.UpdateFromPath(&sc.Env)(ctx)
		if err != nil {
			return nil, err
		}
	}
	sc.Env = mergeMaps(sc.Env, sc.Step.GetEnv()) // step env should not be overwritten
	evaluator := sc.NewExpressionEvaluator()
	sc.interpolateEnv(evaluator)

	common.Logger(ctx).Debugf("setupEnv => %v", sc.Env)
	return evaluator, nil
}

func (sc *StepContext) setupWorkingDirectory() {
	rc := sc.RunContext
	step := sc.Step

	if step.WorkingDirectory == "" {
		step.WorkingDirectory = rc.Run.Job().Defaults.Run.WorkingDirectory
	}

	// jobs can receive context values, so we interpolate
	step.WorkingDirectory = rc.ExprEval.Interpolate(step.WorkingDirectory)

	// but top level keys in workflow file like `defaults` or `env` can't
	if step.WorkingDirectory == "" {
		step.WorkingDirectory = rc.Run.Workflow.Defaults.Run.WorkingDirectory
	}
}

func (sc *StepContext) setupShell() {
	rc := sc.RunContext
	step := sc.Step

	if step.Shell == "" {
		step.Shell = rc.Run.Job().Defaults.Run.Shell
	}

	step.Shell = rc.ExprEval.Interpolate(step.Shell)

	if step.Shell == "" {
		step.Shell = rc.Run.Workflow.Defaults.Run.Shell
	}

	// current GitHub Runner behaviour is that default is `sh`,
	// but if it's not container it validates with `which` command
	// if `bash` is available, and provides `bash` if it is
	// for now I'm going to leave below logic, will address it in different PR
	// https://github.com/actions/runner/blob/9a829995e02d2db64efb939dc2f283002595d4d9/src/Runner.Worker/Handlers/ScriptHandler.cs#L87-L91
	if rc.Run.Job().Container() != nil {
		if rc.Run.Job().Container().Image != "" && step.Shell == "" {
			step.Shell = "sh"
		}
	}
}

func getScriptName(rc *RunContext, step *model.Step) string {
	scriptName := step.ID
	for rcs := rc; rcs.Parent != nil; rcs = rcs.Parent {
		scriptName = fmt.Sprintf("%s-composite-%s", rcs.Parent.CurrentStep, scriptName)
	}
	return fmt.Sprintf("workflow/%s", scriptName)
}

// TODO: Currently we just ignore top level keys, BUT we should return proper error on them
// BUTx2 I leave this for when we rewrite act to use actionlint for workflow validation
// so we return proper errors before any execution or spawning containers
// it will error anyway with:
// OCI runtime exec failed: exec failed: container_linux.go:380: starting container process caused: exec: "${{": executable file not found in $PATH: unknown
func (sc *StepContext) setupShellCommand() (name, script string, err error) {
	sc.setupShell()
	sc.setupWorkingDirectory()

	step := sc.Step

	script = sc.RunContext.ExprEval.Interpolate(step.Run)

	scCmd := step.ShellCommand()

	name = getScriptName(sc.RunContext, step)

	// Reference: https://github.com/actions/runner/blob/8109c962f09d9acc473d92c595ff43afceddb347/src/Runner.Worker/Handlers/ScriptHandlerHelpers.cs#L47-L64
	// Reference: https://github.com/actions/runner/blob/8109c962f09d9acc473d92c595ff43afceddb347/src/Runner.Worker/Handlers/ScriptHandlerHelpers.cs#L19-L27
	runPrepend := ""
	runAppend := ""
	switch step.Shell {
	case "bash", "sh":
		name += ".sh"
	case "pwsh", "powershell":
		name += ".ps1"
		runPrepend = "$ErrorActionPreference = 'stop'"
		runAppend = "if ((Test-Path -LiteralPath variable:/LASTEXITCODE)) { exit $LASTEXITCODE }"
	case "cmd":
		name += ".cmd"
		runPrepend = "@echo off"
	case "python":
		name += ".py"
	}

	script = fmt.Sprintf("%s\n%s\n%s", runPrepend, script, runAppend)

	log.Debugf("Wrote command \n%s\n to '%s'", script, name)

	scriptPath := fmt.Sprintf("%s/%s", ActPath, name)
	sc.Cmd, err = shellquote.Split(strings.Replace(scCmd, `{0}`, scriptPath, 1))

	return name, script, err
}

func (sc *StepContext) setupShellCommandExecutor() common.Executor {
	rc := sc.RunContext
	return func(ctx context.Context) error {
		scriptName, script, err := sc.setupShellCommand()
		if err != nil {
			return err
		}

		return rc.JobContainer.Copy(ActPath, &container.FileEntry{
			Name: scriptName,
			Mode: 0755,
			Body: script,
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

	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TOOL_CACHE", "/opt/hostedtoolcache"))
	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_OS", "Linux"))
	envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TEMP", "/tmp"))

	binds, mounts := rc.GetBindsAndMounts()

	stepContainer := container.NewContainer(&container.NewContainerInput{
		Cmd:         cmd,
		Entrypoint:  entrypoint,
		WorkingDir:  rc.Config.ContainerWorkdir(),
		Image:       image,
		Username:    rc.Config.Secrets["DOCKER_USERNAME"],
		Password:    rc.Config.Secrets["DOCKER_PASSWORD"],
		Name:        createContainerName(rc.jobContainerName(), step.ID),
		Env:         envList,
		Mounts:      mounts,
		NetworkMode: fmt.Sprintf("container:%s", rc.jobContainerName()),
		Binds:       binds,
		Stdout:      logWriter,
		Stderr:      logWriter,
		Privileged:  rc.Config.Privileged,
		UsernsMode:  rc.Config.UsernsMode,
		Platform:    rc.Config.ContainerArchitecture,
	})
	return stepContainer
}

func (sc *StepContext) runUsesContainer() common.Executor {
	rc := sc.RunContext
	step := sc.Step
	return func(ctx context.Context) error {
		image := strings.TrimPrefix(step.Uses, "docker://")
		eval := sc.RunContext.NewExpressionEvaluator()
		cmd, err := shellquote.Split(eval.Interpolate(step.With["args"]))
		if err != nil {
			return err
		}
		entrypoint := strings.Fields(eval.Interpolate(step.With["entrypoint"]))
		stepContainer := sc.newStepContainer(ctx, image, cmd, entrypoint)

		return common.NewPipelineExecutor(
			stepContainer.Pull(rc.Config.ForcePull),
			stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
			stepContainer.Create(rc.Config.ContainerCapAdd, rc.Config.ContainerCapDrop),
			stepContainer.Start(true),
		).Finally(
			stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
		).Finally(stepContainer.Close())(ctx)
	}
}

//go:embed res/trampoline.js
var trampoline embed.FS

func (sc *StepContext) setupAction(actionDir string, actionPath string, localAction bool) common.Executor {
	return func(ctx context.Context) error {
		var readFile func(filename string) (io.Reader, io.Closer, error)
		if localAction {
			_, cpath := sc.getContainerActionPaths(sc.Step, path.Join(actionDir, actionPath), sc.RunContext)
			readFile = func(filename string) (io.Reader, io.Closer, error) {
				tars, err := sc.RunContext.JobContainer.GetContainerArchive(ctx, path.Join(cpath, filename))
				if err != nil {
					return nil, nil, os.ErrNotExist
				}
				treader := tar.NewReader(tars)
				if _, err := treader.Next(); err != nil {
					return nil, nil, os.ErrNotExist
				}
				return treader, tars, nil
			}
		} else {
			readFile = func(filename string) (io.Reader, io.Closer, error) {
				f, err := os.Open(filepath.Join(actionDir, actionPath, filename))
				return f, f, err
			}
		}

		reader, closer, err := readFile("action.yml")
		if os.IsNotExist(err) {
			reader, closer, err = readFile("action.yaml")
			if err != nil {
				if _, closer, err2 := readFile("Dockerfile"); err2 == nil {
					closer.Close()
					sc.Action = &model.Action{
						Name: "(Synthetic)",
						Runs: model.ActionRuns{
							Using: "docker",
							Image: "Dockerfile",
						},
					}
					log.Debugf("Using synthetic action %v for Dockerfile", sc.Action)
					return nil
				}
				if sc.Step.With != nil {
					if val, ok := sc.Step.With["args"]; ok {
						var b []byte
						if b, err = trampoline.ReadFile("res/trampoline.js"); err != nil {
							return err
						}
						err2 := ioutil.WriteFile(filepath.Join(actionDir, actionPath, "trampoline.js"), b, 0400)
						if err2 != nil {
							return err
						}
						sc.Action = &model.Action{
							Name: "(Synthetic)",
							Inputs: map[string]model.Input{
								"cwd": {
									Description: "(Actual working directory)",
									Required:    false,
									Default:     filepath.Join(actionDir, actionPath),
								},
								"command": {
									Description: "(Actual program)",
									Required:    false,
									Default:     val,
								},
							},
							Runs: model.ActionRuns{
								Using: "node12",
								Main:  "trampoline.js",
							},
						}
						log.Debugf("Using synthetic action %v", sc.Action)
						return nil
					}
				}
				return err
			}
		} else if err != nil {
			return err
		}
		defer closer.Close()

		sc.Action, err = model.ReadAction(reader)
		log.Debugf("Read action %v from '%s'", sc.Action, "Unknown")
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
	if step.Type() != model.StepTypeUsesActionRemote {
		actionName = getOsSafeRelativePath(actionDir, rc.Config.Workdir)
		containerActionDir = rc.Config.ContainerWorkdir() + "/" + actionName
		actionName = "./" + actionName
	} else if step.Type() == model.StepTypeUsesActionRemote {
		actionName = getOsSafeRelativePath(actionDir, rc.ActionCacheDir())
		containerActionDir = ActPath + "/actions/" + actionName
	}

	if actionName == "" {
		actionName = filepath.Base(actionDir)
		if runtime.GOOS == "windows" {
			actionName = strings.ReplaceAll(actionName, "\\", "/")
		}
	}
	return actionName, containerActionDir
}

func (sc *StepContext) runAction(actionDir string, actionPath string, actionRepository string, actionRef string, localAction bool) common.Executor {
	rc := sc.RunContext
	step := sc.Step
	return func(ctx context.Context) error {
		// Backup the parent composite action path and restore it on continue
		parentActionPath := rc.ActionPath
		parentActionRepository := rc.ActionRepository
		parentActionRef := rc.ActionRef
		defer func() {
			rc.ActionPath = parentActionPath
			rc.ActionRef = parentActionRef
			rc.ActionRepository = parentActionRepository
		}()
		rc.ActionRef = actionRef
		rc.ActionRepository = actionRepository
		action := sc.Action
		log.Debugf("About to run action %v", action)
		sc.populateEnvsFromInput(action, rc)
		actionLocation := ""
		if actionPath != "" {
			actionLocation = path.Join(actionDir, actionPath)
		} else {
			actionLocation = actionDir
		}
		actionName, containerActionDir := sc.getContainerActionPaths(step, actionLocation, rc)

		log.Debugf("type=%v actionDir=%s actionPath=%s workdir=%s actionCacheDir=%s actionName=%s containerActionDir=%s", step.Type(), actionDir, actionPath, rc.Config.Workdir, rc.ActionCacheDir(), actionName, containerActionDir)

		maybeCopyToActionDir := func() error {
			rc.ActionPath = containerActionDir
			if step.Type() != model.StepTypeUsesActionRemote {
				return nil
			}
			if err := removeGitIgnore(actionDir); err != nil {
				return err
			}

			var containerActionDirCopy string
			containerActionDirCopy = strings.TrimSuffix(containerActionDir, actionPath)
			log.Debug(containerActionDirCopy)

			if !strings.HasSuffix(containerActionDirCopy, `/`) {
				containerActionDirCopy += `/`
			}
			return rc.JobContainer.CopyDir(containerActionDirCopy, actionDir+"/", rc.Config.UseGitIgnore)(ctx)
		}

		switch action.Runs.Using {
		case model.ActionRunsUsingNode12, model.ActionRunsUsingNode16:
			if err := maybeCopyToActionDir(); err != nil {
				return err
			}
			containerArgs := []string{"node", path.Join(containerActionDir, action.Runs.Main)}
			log.Debugf("executing remote job container: %s", containerArgs)
			return rc.execJobContainer(containerArgs, sc.Env, "", "")(ctx)
		case model.ActionRunsUsingDocker:
			return sc.execAsDocker(ctx, action, actionName, containerActionDir, actionLocation, rc, step, localAction)
		case model.ActionRunsUsingComposite:
			return sc.execAsComposite(ctx, step, actionDir, rc, containerActionDir, actionName, actionPath, action, maybeCopyToActionDir)
		default:
			return fmt.Errorf(fmt.Sprintf("The runs.using key must be one of: %v, got %s", []string{
				model.ActionRunsUsingDocker,
				model.ActionRunsUsingNode12,
				model.ActionRunsUsingNode16,
				model.ActionRunsUsingComposite,
			}, action.Runs.Using))
		}
	}
}

func (sc *StepContext) evalDockerArgs(action *model.Action, cmd *[]string) {
	rc := sc.RunContext
	step := sc.Step
	oldInputs := rc.Inputs
	defer func() {
		rc.Inputs = oldInputs
	}()
	inputs := make(map[string]interface{})
	eval := sc.RunContext.NewExpressionEvaluator()
	// Set Defaults
	for k, input := range action.Inputs {
		inputs[k] = eval.Interpolate(input.Default)
	}
	if step.With != nil {
		for k, v := range step.With {
			inputs[k] = eval.Interpolate(v)
		}
	}
	rc.Inputs = inputs
	stepEE := sc.NewExpressionEvaluator()
	for i, v := range *cmd {
		(*cmd)[i] = stepEE.Interpolate(v)
	}
	sc.Env = mergeMaps(sc.Env, action.Runs.Env)

	ee := sc.NewExpressionEvaluator()
	for k, v := range sc.Env {
		sc.Env[k] = ee.Interpolate(v)
	}
}

// TODO: break out parts of function to reduce complexicity
// nolint:gocyclo
func (sc *StepContext) execAsDocker(ctx context.Context, action *model.Action, actionName string, containerLocation string, actionLocation string, rc *RunContext, step *model.Step, localAction bool) error {
	var prepImage common.Executor
	var image string
	if strings.HasPrefix(action.Runs.Image, "docker://") {
		image = strings.TrimPrefix(action.Runs.Image, "docker://")
	} else {
		// "-dockeraction" enshures that "./", "./test " won't get converted to "act-:latest", "act-test-:latest" which are invalid docker image names
		image = fmt.Sprintf("%s-dockeraction:%s", regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(actionName, "-"), "latest")
		image = fmt.Sprintf("act-%s", strings.TrimLeft(image, "-"))
		image = strings.ToLower(image)
		basedir := actionLocation
		if localAction {
			basedir = containerLocation
		}
		contextDir := filepath.Join(basedir, action.Runs.Main)

		anyArchExists, err := container.ImageExistsLocally(ctx, image, "any")
		if err != nil {
			return err
		}

		correctArchExists, err := container.ImageExistsLocally(ctx, image, rc.Config.ContainerArchitecture)
		if err != nil {
			return err
		}

		if anyArchExists && !correctArchExists {
			wasRemoved, err := container.RemoveImage(ctx, image, true, true)
			if err != nil {
				return err
			}
			if !wasRemoved {
				return fmt.Errorf("failed to remove image '%s'", image)
			}
		}

		if !correctArchExists || rc.Config.ForceRebuild {
			log.Debugf("image '%s' for architecture '%s' will be built from context '%s", image, rc.Config.ContainerArchitecture, contextDir)
			var actionContainer container.Container
			if localAction {
				actionContainer = sc.RunContext.JobContainer
			}
			prepImage = container.NewDockerBuildExecutor(container.NewDockerBuildExecutorInput{
				ContextDir: contextDir,
				ImageTag:   image,
				Container:  actionContainer,
				Platform:   rc.Config.ContainerArchitecture,
			})
		} else {
			log.Debugf("image '%s' for architecture '%s' already exists", image, rc.Config.ContainerArchitecture)
		}
	}
	eval := sc.NewExpressionEvaluator()
	cmd, err := shellquote.Split(eval.Interpolate(step.With["args"]))
	if err != nil {
		return err
	}
	if len(cmd) == 0 {
		cmd = action.Runs.Args
		sc.evalDockerArgs(action, &cmd)
	}
	entrypoint := strings.Fields(eval.Interpolate(step.With["entrypoint"]))
	if len(entrypoint) == 0 {
		if action.Runs.Entrypoint != "" {
			entrypoint, err = shellquote.Split(action.Runs.Entrypoint)
			if err != nil {
				return err
			}
		} else {
			entrypoint = nil
		}
	}
	stepContainer := sc.newStepContainer(ctx, image, cmd, entrypoint)
	return common.NewPipelineExecutor(
		prepImage,
		stepContainer.Pull(rc.Config.ForcePull),
		stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
		stepContainer.Create(rc.Config.ContainerCapAdd, rc.Config.ContainerCapDrop),
		stepContainer.Start(true),
	).Finally(
		stepContainer.Remove().IfBool(!rc.Config.ReuseContainers),
	).Finally(stepContainer.Close())(ctx)
}

func (sc *StepContext) execAsComposite(ctx context.Context, step *model.Step, _ string, rc *RunContext, containerActionDir string, actionName string, _ string, action *model.Action, maybeCopyToActionDir func() error) error {
	err := maybeCopyToActionDir()

	if err != nil {
		return err
	}
	// Disable some features of composite actions, only for feature parity with github
	for _, compositeStep := range action.Runs.Steps {
		if err := compositeStep.Validate(rc.Config.CompositeRestrictions); err != nil {
			return err
		}
	}
	inputs := make(map[string]interface{})
	eval := sc.RunContext.NewExpressionEvaluator()
	// Set Defaults
	for k, input := range action.Inputs {
		inputs[k] = eval.Interpolate(input.Default)
	}
	if step.With != nil {
		for k, v := range step.With {
			inputs[k] = eval.Interpolate(v)
		}
	}
	// Doesn't work with the command processor has a pointer to the original rc
	// compositerc := rc.Clone()
	// Workaround start
	backup := *rc
	defer func() { *rc = backup }()
	*rc = *rc.Clone()
	scriptName := backup.CurrentStep
	for rcs := &backup; rcs.Parent != nil; rcs = rcs.Parent {
		scriptName = fmt.Sprintf("%s-composite-%s", rcs.Parent.CurrentStep, scriptName)
	}
	compositerc := rc
	compositerc.Parent = &RunContext{
		CurrentStep: scriptName,
	}
	// Workaround end
	compositerc.Composite = action
	envToEvaluate := mergeMaps(compositerc.Env, step.Environment())
	compositerc.Env = make(map[string]string)
	// origEnvMap: is used to pass env changes back to parent runcontext
	origEnvMap := make(map[string]string)
	for k, v := range envToEvaluate {
		ev := eval.Interpolate(v)
		origEnvMap[k] = ev
		compositerc.Env[k] = ev
	}
	compositerc.Inputs = inputs
	compositerc.ExprEval = compositerc.NewExpressionEvaluator()
	err = compositerc.CompositeExecutor()(ctx)

	// Map outputs to parent rc
	eval = (&StepContext{
		Env:        compositerc.Env,
		RunContext: compositerc,
	}).NewExpressionEvaluator()
	for outputName, output := range action.Outputs {
		backup.setOutput(ctx, map[string]string{
			"name": outputName,
		}, eval.Interpolate(output.Value))
	}
	// Test if evaluated parent env was altered by this composite step
	// Known Issues:
	// - you try to set an env variable to the same value as a scoped step env, will be discared
	for k, v := range compositerc.Env {
		if ov, ok := origEnvMap[k]; !ok || ov != v {
			backup.Env[k] = v
		}
	}
	return err
}

func (sc *StepContext) populateEnvsFromInput(action *model.Action, rc *RunContext) {
	for inputID, input := range action.Inputs {
		envKey := regexp.MustCompile("[^A-Z0-9-]").ReplaceAllString(strings.ToUpper(inputID), "_")
		envKey = fmt.Sprintf("INPUT_%s", envKey)
		if _, ok := sc.Env[envKey]; !ok {
			sc.Env[envKey] = rc.ExprEval.Interpolate(input.Default)
		}
	}
}

type remoteAction struct {
	URL  string
	Org  string
	Repo string
	Path string
	Ref  string
}

func (ra *remoteAction) CloneURL() string {
	return fmt.Sprintf("https://%s/%s/%s", ra.URL, ra.Org, ra.Repo)
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
		URL:  "github.com",
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
