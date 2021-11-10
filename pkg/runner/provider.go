package runner

import (
	"archive/tar"
	"context"
	"embed"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	// Go told me to?
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

type ActionProvider interface {
	SetupAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor
	RunAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor
	ExecuteNode12Action(sc *StepContext, containerActionDir string, ctx context.Context, maybeCopyToActionDir func() error) error
	ExecuteNode12PostAction(sc *StepContext, containerActionDir string, ctx context.Context) error
}

type actProvider struct{}

func (a *actProvider) ExecuteNode12Action(sc *StepContext, containerActionDir string, ctx context.Context, maybeCopyToActionDir func() error) error {
	if err := maybeCopyToActionDir(); err != nil {
		return err
	}
	action := sc.Action
	containerArgs := []string{"node", path.Join(containerActionDir, action.Runs.Main)}
	log.Debugf("executing remote job container: %s", containerArgs)
	rc := sc.RunContext
	return rc.execJobContainer(containerArgs, sc.Env, "", "")(ctx)
}

func (a *actProvider) ExecuteNode12PostAction(sc *StepContext, containerActionDir string, ctx context.Context) error {
	action := sc.Action
	containerArgs := []string{"node", path.Join(containerActionDir, action.Runs.Post)}
	log.Debugf("executing remote job container: %s", containerArgs)

	rc := sc.RunContext
	env := mergeMaps(rc.Env, sc.Env)
	return rc.execJobContainer(containerArgs, env, "", "")(ctx)
}

//go:embed res/trampoline.js
var trampoline embed.FS

func (a *actProvider) SetupAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor {
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

func (a *actProvider) RunAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor {
	return RunAction(sc, actionDir, actionPath, localAction)
}

func appendPostAction(sc *StepContext, containerActionDir string, mainErr error) {
	rc := sc.RunContext
	rc.PostActionExecutor = append(rc.PostActionExecutor, func(ctx context.Context) error {
		action := sc.Action
		if mainErr != nil {
			log.Warningf("Skipping post action: %s due to main action failure", action.Name)
			return nil
		}
		runPost, err := rc.EvalBool(action.Runs.PostIf)
		if err != nil {
			common.Logger(ctx).Errorf("Error in post-if: expression - %s", action.Name)
			return err
		}

		if !runPost {
			log.Debugf("Skipping post action '%s' due to '%s' condittion", action.Name, action.Runs.PostIf)
			return nil
		}
		return rc.Providers.Action.ExecuteNode12PostAction(sc, containerActionDir, ctx)
	})
}

func RunAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor {
	rc := sc.RunContext
	step := sc.Step
	return func(ctx context.Context) error {
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

		sc.Env = mergeMaps(sc.Env, action.Runs.Env)

		log.Debugf("type=%v actionDir=%s actionPath=%s workdir=%s actionCacheDir=%s actionName=%s containerActionDir=%s", step.Type(), actionDir, actionPath, rc.Config.Workdir, rc.ActionCacheDir(), actionName, containerActionDir)

		maybeCopyToActionDir := func() error {
			sc.Env["GITHUB_ACTION_PATH"] = containerActionDir
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
		case model.ActionRunsUsingNode12:
			provider := sc.RunContext.Providers.Action
			mainErr := provider.ExecuteNode12Action(sc, containerActionDir, ctx, maybeCopyToActionDir)
			if action.Runs.Post != "" {
				appendPostAction(sc, containerActionDir, mainErr)
			}
			return mainErr
		case model.ActionRunsUsingDocker:
			return sc.execAsDocker(ctx, action, actionName, containerActionDir, actionLocation, rc, step, localAction)
		case model.ActionRunsUsingComposite:
			return sc.execAsComposite(ctx, step, actionDir, rc, containerActionDir, actionName, actionPath, action, maybeCopyToActionDir)
		default:
			return fmt.Errorf(fmt.Sprintf("The runs.using key must be one of: %v, got %s", []string{
				model.ActionRunsUsingDocker,
				model.ActionRunsUsingNode12,
				model.ActionRunsUsingComposite,
			}, action.Runs.Using))
		}
	}
}

func NewActionProvider() ActionProvider {
	return &actProvider{}
}
