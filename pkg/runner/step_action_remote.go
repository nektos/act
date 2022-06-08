package runner

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/pkg/errors"
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
}

var (
	stepActionRemoteNewCloneExecutor = common.NewGitCloneExecutor
)

func (sar *stepActionRemote) prepareActionExecutor() common.Executor {
	return common.NewPipelineExecutor(
		func(ctx context.Context) error {
			if sar.remoteAction != nil && sar.action != nil {
				// we are already good to run
				return nil
			}

			sar.remoteAction = newRemoteAction(sar.Step.Uses)
			if sar.remoteAction == nil {
				return fmt.Errorf("Expected format {org}/{repo}[/path]@ref. Actual '%s' Input string was not in a correct format", sar.Step.Uses)
			}

			sar.remoteAction.URL = sar.RunContext.Config.GitHubInstance

			github := sar.RunContext.getGithubContext()
			if sar.remoteAction.IsCheckout() && isLocalCheckout(github, sar.Step) && !sar.RunContext.Config.NoSkipCheckout {
				common.Logger(ctx).Debugf("Skipping local actions/checkout because workdir was already copied")
				return nil
			}

			actionDir := fmt.Sprintf("%s/%s", sar.RunContext.ActionCacheDir(), strings.ReplaceAll(sar.Step.Uses, "/", "-"))
			gitClone := stepActionRemoteNewCloneExecutor(common.NewGitCloneExecutorInput{
				URL:   sar.remoteAction.CloneURL(),
				Ref:   sar.remoteAction.Ref,
				Dir:   actionDir,
				Token: github.Token,
			})
			var ntErr common.Executor
			if err := gitClone(ctx); err != nil {
				if err.Error() == "short SHA references are not supported" {
					err = errors.Cause(err)
					return fmt.Errorf("Unable to resolve action `%s`, the provided ref `%s` is the shortened version of a commit SHA, which is not supported. Please use the full commit SHA `%s` instead", sar.Step.Uses, sar.remoteAction.Ref, err.Error())
				} else if err.Error() != "some refs were not updated" {
					return err
				} else {
					ntErr = common.NewInfoExecutor("Non-terminating error while running 'git clone': %v", err)
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
					actionModel, err := sar.readAction(sar.Step, actionDir, sar.remoteAction.Path, remoteReader(ctx), ioutil.WriteFile)
					sar.action = actionModel
					return err
				},
			)(ctx)
		},
		func(ctx context.Context) error {
			sar.RunContext.setupActionInputs(sar)
			return nil
		})
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
			github := sar.RunContext.getGithubContext()
			if sar.remoteAction.IsCheckout() && isLocalCheckout(github, sar.Step) && !sar.RunContext.Config.NoSkipCheckout {
				common.Logger(ctx).Debugf("Skipping local actions/checkout because workdir was already copied")
				return nil
			}

			actionDir := fmt.Sprintf("%s/%s", sar.RunContext.ActionCacheDir(), strings.ReplaceAll(sar.Step.Uses, "/", "-"))

			return common.NewPipelineExecutor(
				sar.runAction(sar, actionDir, sar.remoteAction),
			)(ctx)
		}),
	)
}

func (sar *stepActionRemote) post() common.Executor {
	return runStepExecutor(sar, stepStagePost, runPostStep(sar)).If(hasPostStep(sar)).If(shouldRunPostStep(sar))
}

func (sar *stepActionRemote) getRunContext() *RunContext {
	return sar.RunContext
}

func (sar *stepActionRemote) getStepModel() *model.Step {
	return sar.Step
}

func (sar *stepActionRemote) getEnv() *map[string]string {
	return &sar.env
}

func (sar *stepActionRemote) getIfExpression(stage stepStage) string {
	switch stage {
	case stepStagePre:
		github := sar.RunContext.getGithubContext()
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

func (sar *stepActionRemote) getCompositeRunContext() *RunContext {
	if sar.compositeRunContext == nil {
		actionDir := fmt.Sprintf("%s/%s", sar.RunContext.ActionCacheDir(), strings.ReplaceAll(sar.Step.Uses, "/", "-"))
		actionLocation := path.Join(actionDir, sar.remoteAction.Path)
		_, containerActionDir := getContainerActionPaths(sar.getStepModel(), actionLocation, sar.RunContext)

		sar.compositeRunContext = newCompositeRunContext(sar.RunContext, sar, containerActionDir)
		sar.compositeSteps = sar.compositeRunContext.compositeExecutor(sar.action)
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
