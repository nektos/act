package runner

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	gogit "github.com/go-git/go-git/v5"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/common/git"
	"github.com/nektos/act/pkg/model"
)

type stepActionRemote struct {
	Step                *model.Step
	RunContext          *RunContext
	compositeRunContext *RunContext
	compositeSteps      *compositeSteps
	readAction          readAction
	runAction           runAction
	action              *model.Action
	env                 map[string]string
	remoteAction        *remoteAction
	cacheDir            string
	resolvedSha         string
}

var (
	stepActionRemoteNewCloneExecutor = git.NewGitCloneExecutor
)

func (sar *stepActionRemote) prepareActionExecutor() common.Executor {
	return func(ctx context.Context) error {
		if sar.remoteAction != nil && sar.action != nil {
			// we are already good to run
			return nil
		}

		sar.remoteAction = newRemoteAction(sar.Step.Uses)
		if sar.remoteAction == nil {
			return fmt.Errorf("Expected format {org}/{repo}[/path]@ref. Actual '%s' Input string was not in a correct format", sar.Step.Uses)
		}

		github := sar.getGithubContext(ctx)
		sar.remoteAction.URL = github.ServerURL

		if sar.remoteAction.IsCheckout() && isLocalCheckout(github, sar.Step) && !sar.RunContext.Config.NoSkipCheckout {
			common.Logger(ctx).Debugf("Skipping local actions/checkout because workdir was already copied")
			return nil
		}

		for _, action := range sar.RunContext.Config.ReplaceGheActionWithGithubCom {
			if strings.EqualFold(fmt.Sprintf("%s/%s", sar.remoteAction.Org, sar.remoteAction.Repo), action) {
				sar.remoteAction.URL = "https://github.com"
				github.Token = sar.RunContext.Config.ReplaceGheActionTokenWithGithubCom
			}
		}
		if sar.RunContext.Config.ActionCache != nil {
			cache := sar.RunContext.Config.ActionCache

			var err error
			sar.cacheDir = fmt.Sprintf("%s/%s", sar.remoteAction.Org, sar.remoteAction.Repo)
			sar.resolvedSha, err = cache.Fetch(ctx, sar.cacheDir, sar.remoteAction.URL+"/"+sar.cacheDir, sar.remoteAction.Ref, github.Token)
			if err != nil {
				return err
			}

			remoteReader := func(ctx context.Context) actionYamlReader {
				return func(filename string) (io.Reader, io.Closer, error) {
					spath := filename
					for i := 0; i < maxSymlinkDepth; i++ {
						tars, err := cache.GetTarArchive(ctx, sar.cacheDir, sar.resolvedSha, spath)
						if err != nil {
							return nil, nil, os.ErrNotExist
						}
						treader := tar.NewReader(tars)
						header, err := treader.Next()
						if err != nil {
							return nil, nil, os.ErrNotExist
						}
						if header.FileInfo().Mode()&os.ModeSymlink == os.ModeSymlink {
							spath, err = symlinkJoin(spath, header.Linkname, ".")
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

			actionModel, err := sar.readAction(ctx, sar.Step, sar.resolvedSha, sar.remoteAction.Path, remoteReader(ctx), os.WriteFile)
			sar.action = actionModel
			return err
		}

		actionDir := fmt.Sprintf("%s/%s", sar.RunContext.ActionCacheDir(), safeFilename(sar.Step.Uses))
		gitClone := stepActionRemoteNewCloneExecutor(git.NewGitCloneExecutorInput{
			URL:         sar.remoteAction.CloneURL(),
			Ref:         sar.remoteAction.Ref,
			Dir:         actionDir,
			Token:       github.Token,
			OfflineMode: sar.RunContext.Config.ActionOfflineMode,
		})
		var ntErr common.Executor
		if err := gitClone(ctx); err != nil {
			if errors.Is(err, git.ErrShortRef) {
				return fmt.Errorf("Unable to resolve action `%s`, the provided ref `%s` is the shortened version of a commit SHA, which is not supported. Please use the full commit SHA `%s` instead",
					sar.Step.Uses, sar.remoteAction.Ref, err.(*git.Error).Commit())
			} else if errors.Is(err, gogit.ErrForceNeeded) { // TODO: figure out if it will be easy to shadow/alias go-git err's
				ntErr = common.NewInfoExecutor("Non-terminating error while running 'git clone': %v", err)
			} else {
				return err
			}
		}

		remoteReader := func(ctx context.Context) actionYamlReader {
			return func(filename string) (io.Reader, io.Closer, error) {
				f, err := os.Open(filepath.Join(actionDir, sar.remoteAction.Path, filename))
				return f, f, err
			}
		}

		return common.NewPipelineExecutor(
			ntErr,
			func(ctx context.Context) error {
				actionModel, err := sar.readAction(ctx, sar.Step, actionDir, sar.remoteAction.Path, remoteReader(ctx), os.WriteFile)
				sar.action = actionModel
				return err
			},
		)(ctx)
	}
}

func (sar *stepActionRemote) pre() common.Executor {
	sar.env = map[string]string{}

	return common.NewPipelineExecutor(
		sar.prepareActionExecutor(),
		runStepExecutor(sar, stepStagePre, runPreStep(sar)).If(hasPreStep(sar)).If(shouldRunPreStep(sar)))
}

func (sar *stepActionRemote) main() common.Executor {
	return common.NewPipelineExecutor(
		sar.prepareActionExecutor(),
		runStepExecutor(sar, stepStageMain, func(ctx context.Context) error {
			github := sar.getGithubContext(ctx)
			if sar.remoteAction.IsCheckout() && isLocalCheckout(github, sar.Step) && !sar.RunContext.Config.NoSkipCheckout {
				if sar.RunContext.Config.BindWorkdir {
					common.Logger(ctx).Debugf("Skipping local actions/checkout because you bound your workspace")
					return nil
				}
				eval := sar.RunContext.NewExpressionEvaluator(ctx)
				copyToPath := path.Join(sar.RunContext.JobContainer.ToContainerPath(sar.RunContext.Config.Workdir), eval.Interpolate(ctx, sar.Step.With["path"]))
				return sar.RunContext.JobContainer.CopyDir(copyToPath, sar.RunContext.Config.Workdir+string(filepath.Separator)+".", sar.RunContext.Config.UseGitIgnore)(ctx)
			}

			actionDir := fmt.Sprintf("%s/%s", sar.RunContext.ActionCacheDir(), safeFilename(sar.Step.Uses))

			return sar.runAction(sar, actionDir, sar.remoteAction)(ctx)
		}),
	)
}

func (sar *stepActionRemote) post() common.Executor {
	return runStepExecutor(sar, stepStagePost, runPostStep(sar)).If(hasPostStep(sar)).If(shouldRunPostStep(sar))
}

func (sar *stepActionRemote) getRunContext() *RunContext {
	return sar.RunContext
}

func (sar *stepActionRemote) getGithubContext(ctx context.Context) *model.GithubContext {
	ghc := sar.getRunContext().getGithubContext(ctx)

	// extend github context if we already have an initialized remoteAction
	remoteAction := sar.remoteAction
	if remoteAction != nil {
		ghc.ActionRepository = fmt.Sprintf("%s/%s", remoteAction.Org, remoteAction.Repo)
		ghc.ActionRef = remoteAction.Ref
	}

	return ghc
}

func (sar *stepActionRemote) getStepModel() *model.Step {
	return sar.Step
}

func (sar *stepActionRemote) getEnv() *map[string]string {
	return &sar.env
}

func (sar *stepActionRemote) getIfExpression(ctx context.Context, stage stepStage) string {
	switch stage {
	case stepStagePre:
		github := sar.getGithubContext(ctx)
		if sar.remoteAction.IsCheckout() && isLocalCheckout(github, sar.Step) && !sar.RunContext.Config.NoSkipCheckout {
			// skip local checkout pre step
			return "false"
		}
		return sar.action.Runs.PreIf
	case stepStageMain:
		return sar.Step.If.Value
	case stepStagePost:
		return sar.action.Runs.PostIf
	}
	return ""
}

func (sar *stepActionRemote) getActionModel() *model.Action {
	return sar.action
}

func (sar *stepActionRemote) getCompositeRunContext(ctx context.Context) *RunContext {
	if sar.compositeRunContext == nil {
		actionDir := fmt.Sprintf("%s/%s", sar.RunContext.ActionCacheDir(), safeFilename(sar.Step.Uses))
		actionLocation := path.Join(actionDir, sar.remoteAction.Path)
		_, containerActionDir := getContainerActionPaths(sar.getStepModel(), actionLocation, sar.RunContext)

		sar.compositeRunContext = newCompositeRunContext(ctx, sar.RunContext, sar, containerActionDir)
		sar.compositeSteps = sar.compositeRunContext.compositeExecutor(sar.action)
	} else {
		// Re-evaluate environment here. For remote actions the environment
		// need to be re-created for every stage (pre, main, post) as there
		// might be required context changes (inputs/outputs) while the action
		// stages are executed. (e.g. the output of another action is the
		// input for this action during the main stage, but the env
		// was already created during the pre stage)
		env := evaluateCompositeInputAndEnv(ctx, sar.RunContext, sar)
		sar.compositeRunContext.Env = env
		sar.compositeRunContext.ExtraPath = sar.RunContext.ExtraPath
	}
	return sar.compositeRunContext
}

func (sar *stepActionRemote) getCompositeSteps() *compositeSteps {
	return sar.compositeSteps
}

type remoteAction struct {
	URL  string
	Org  string
	Repo string
	Path string
	Ref  string
}

func (ra *remoteAction) CloneURL() string {
	return fmt.Sprintf("%s/%s/%s", ra.URL, ra.Org, ra.Repo)
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
		URL:  "https://github.com",
	}
}

func safeFilename(s string) string {
	return strings.NewReplacer(
		`<`, "-",
		`>`, "-",
		`:`, "-",
		`"`, "-",
		`/`, "-",
		`\`, "-",
		`|`, "-",
		`?`, "-",
		`*`, "-",
	).Replace(s)
}
