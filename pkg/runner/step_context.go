package runner

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/nektos/act/pkg/common"
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

		localReader := func(ctx context.Context) actionyamlReader {
			_, cpath := sc.getContainerActionPaths(sc.Step, path.Join(actionDir, ""), sc.RunContext)
			return func(filename string) (io.Reader, io.Closer, error) {
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
		}

		return common.NewPipelineExecutor(
			sc.setupAction(actionDir, "", localReader),
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

		remoteReader := func(ctx context.Context) actionyamlReader {
			return func(filename string) (io.Reader, io.Closer, error) {
				f, err := os.Open(filepath.Join(actionDir, remoteAction.Path, filename))
				return f, f, err
			}
		}

		return common.NewPipelineExecutor(
			ntErr,
			sc.setupAction(actionDir, remoteAction.Path, remoteReader),
			sc.runAction(actionDir, remoteAction.Path, remoteAction.Repo, remoteAction.Ref, false),
		)
	case model.StepTypeInvalid:
		return common.NewErrorExecutor(fmt.Errorf("Invalid run/uses syntax for job:%s step:%+v", rc.Run, step))
	}

	return common.NewErrorExecutor(fmt.Errorf("Unable to determine how to run job:%s step:%+v", rc.Run, step))
}

func (sc *StepContext) interpolateEnv(exprEval ExpressionEvaluator) {
	for k, v := range sc.Env {
		sc.Env[k] = exprEval.Interpolate(v)
	}
}

func (sc *StepContext) setupAction(actionDir string, actionPath string, reader func(context.Context) actionyamlReader) common.Executor {
	return func(ctx context.Context) error {
		action, err := sc.readAction(sc.Step, actionDir, actionPath, reader(ctx), ioutil.WriteFile)
		sc.Action = action
		log.Debugf("Read action %v from '%s'", sc.Action, "Unknown")
		return err
	}
}
