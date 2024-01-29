package runner

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

type stepActionLocal struct {
	Step                *model.Step
	RunContext          *RunContext
	compositeRunContext *RunContext
	compositeSteps      *compositeSteps
	runAction           runAction
	readAction          readAction
	env                 map[string]string
	action              *model.Action
}

func (sal *stepActionLocal) pre() common.Executor {
	sal.env = map[string]string{}

	return func(ctx context.Context) error {
		return nil
	}
}

func (sal *stepActionLocal) main() common.Executor {
	return runStepExecutor(sal, stepStageMain, func(ctx context.Context) error {
		if common.Dryrun(ctx) {
			return nil
		}

		actionDir := filepath.Join(sal.getRunContext().Config.Workdir, sal.Step.Uses)

		localReader := func(ctx context.Context) actionYamlReader {
			_, cpath := getContainerActionPaths(sal.Step, path.Join(actionDir, ""), sal.RunContext)
			return func(filename string) (io.Reader, io.Closer, error) {
				spath := path.Join(cpath, filename)
				for i := 0; i < maxSymlinkDepth; i++ {
					tars, err := sal.RunContext.JobContainer.GetContainerArchive(ctx, spath)
					if errors.Is(err, fs.ErrNotExist) {
						return nil, nil, err
					} else if err != nil {
						return nil, nil, fs.ErrNotExist
					}
					treader := tar.NewReader(tars)
					header, err := treader.Next()
					if errors.Is(err, io.EOF) {
						return nil, nil, os.ErrNotExist
					} else if err != nil {
						return nil, nil, err
					}
					if header.FileInfo().Mode()&os.ModeSymlink == os.ModeSymlink {
						spath, err = symlinkJoin(spath, header.Linkname, cpath)
						if err != nil {
							return nil, nil, err
						}
					} else {
						return treader, tars, nil
					}
				}
				return nil, nil, fmt.Errorf("max depth %d of symlinks exceeded while reading %s", maxSymlinkDepth, spath)
			}
		}

		actionModel, err := sal.readAction(ctx, sal.Step, actionDir, "", localReader(ctx), os.WriteFile)
		if err != nil {
			return err
		}

		sal.action = actionModel

		return sal.runAction(sal, actionDir, nil)(ctx)
	})
}

func (sal *stepActionLocal) post() common.Executor {
	return runStepExecutor(sal, stepStagePost, runPostStep(sal)).If(hasPostStep(sal)).If(shouldRunPostStep(sal))
}

func (sal *stepActionLocal) getRunContext() *RunContext {
	return sal.RunContext
}

func (sal *stepActionLocal) getGithubContext(ctx context.Context) *model.GithubContext {
	return sal.getRunContext().getGithubContext(ctx)
}

func (sal *stepActionLocal) getStepModel() *model.Step {
	return sal.Step
}

func (sal *stepActionLocal) getEnv() *map[string]string {
	return &sal.env
}

func (sal *stepActionLocal) getIfExpression(_ context.Context, stage stepStage) string {
	switch stage {
	case stepStageMain:
		return sal.Step.If.Value
	case stepStagePost:
		return sal.action.Runs.PostIf
	}
	return ""
}

func (sal *stepActionLocal) getActionModel() *model.Action {
	return sal.action
}

func (sal *stepActionLocal) getCompositeRunContext(ctx context.Context) *RunContext {
	if sal.compositeRunContext == nil {
		actionDir := filepath.Join(sal.RunContext.Config.Workdir, sal.Step.Uses)
		_, containerActionDir := getContainerActionPaths(sal.getStepModel(), actionDir, sal.RunContext)

		sal.compositeRunContext = newCompositeRunContext(ctx, sal.RunContext, sal, containerActionDir)
		sal.compositeSteps = sal.compositeRunContext.compositeExecutor(sal.action)
	}
	return sal.compositeRunContext
}

func (sal *stepActionLocal) getCompositeSteps() *compositeSteps {
	return sal.compositeSteps
}
