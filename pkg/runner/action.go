package runner

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

type ActionReader interface {
	readAction(step *model.Step, actionDir string, actionPath string, readFile actionyamlReader) (*model.Action, error)
}

type actionyamlReader func(filename string) (io.Reader, io.Closer, error)
type fileWriter func(filename string, data []byte, perm fs.FileMode) error

//go:embed res/trampoline.js
var trampoline embed.FS

func (sc *StepContext) readAction(step *model.Step, actionDir string, actionPath string, readFile actionyamlReader, writeFile fileWriter) (*model.Action, error) {
	reader, closer, err := readFile("action.yml")
	if os.IsNotExist(err) {
		reader, closer, err = readFile("action.yaml")
		if err != nil {
			if _, closer, err2 := readFile("Dockerfile"); err2 == nil {
				closer.Close()
				action := &model.Action{
					Name: "(Synthetic)",
					Runs: model.ActionRuns{
						Using: "docker",
						Image: "Dockerfile",
					},
				}
				log.Debugf("Using synthetic action %v for Dockerfile", action)
				return action, nil
			}
			if step.With != nil {
				if val, ok := step.With["args"]; ok {
					var b []byte
					if b, err = trampoline.ReadFile("res/trampoline.js"); err != nil {
						return nil, err
					}
					err2 := writeFile(filepath.Join(actionDir, actionPath, "trampoline.js"), b, 0400)
					if err2 != nil {
						return nil, err2
					}
					action := &model.Action{
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
					log.Debugf("Using synthetic action %v", action)
					return action, nil
				}
			}
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	defer closer.Close()

	action, err := model.ReadAction(reader)
	log.Debugf("Read action %v from '%s'", action, "Unknown")
	return action, err
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

	backup.Masks = append(backup.Masks, compositerc.Masks...)
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

func getOsSafeRelativePath(s, prefix string) string {
	actionName := strings.TrimPrefix(s, prefix)
	if runtime.GOOS == "windows" {
		actionName = strings.ReplaceAll(actionName, "\\", "/")
	}
	actionName = strings.TrimPrefix(actionName, "/")

	return actionName
}
