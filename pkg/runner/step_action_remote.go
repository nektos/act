package runner

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type stepActionRemote struct {
	Step       *model.Step
	RunContext *RunContext
	readAction readAction
	runAction  runAction
	action     *model.Action
	env        map[string]string
}

func (sar *stepActionRemote) pre() common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

var (
	stepActionRemoteNewCloneExecutor = common.NewGitCloneExecutor
)

func (sar *stepActionRemote) main() common.Executor {
	sar.env = map[string]string{}

	return runStepExecutor(sar, func(ctx context.Context) error {
		remoteAction := newRemoteAction(sar.Step.Uses)
		if remoteAction == nil {
			return fmt.Errorf("Expected format {org}/{repo}[/path]@ref. Actual '%s' Input string was not in a correct format", sar.Step.Uses)
		}

		remoteAction.URL = sar.RunContext.Config.GitHubInstance

		github := sar.RunContext.getGithubContext()
		if remoteAction.IsCheckout() && isLocalCheckout(github, sar.Step) {
			common.Logger(ctx).Debugf("Skipping local actions/checkout because workdir was already copied")
			return nil
		}

		actionDir := fmt.Sprintf("%s/%s", sar.RunContext.ActionCacheDir(), strings.ReplaceAll(sar.Step.Uses, "/", "-"))
		gitClone := stepActionRemoteNewCloneExecutor(common.NewGitCloneExecutorInput{
			URL:   remoteAction.CloneURL(),
			Ref:   remoteAction.Ref,
			Dir:   actionDir,
			Token: github.Token,
		})
		var ntErr common.Executor
		if err := gitClone(ctx); err != nil {
			if err.Error() == "short SHA references are not supported" {
				err = errors.Cause(err)
				return fmt.Errorf("Unable to resolve action `%s`, the provided ref `%s` is the shortened version of a commit SHA, which is not supported. Please use the full commit SHA `%s` instead", sar.Step.Uses, remoteAction.Ref, err.Error())
			} else if err.Error() != "some refs were not updated" {
				return err
			} else {
				ntErr = common.NewInfoExecutor("Non-terminating error while running 'git clone': %v", err)
			}
		}

		remoteReader := func(ctx context.Context) actionYamlReader {
			return func(filename string) (io.Reader, io.Closer, error) {
				f, err := os.Open(filepath.Join(actionDir, remoteAction.Path, filename))
				return f, f, err
			}
		}

		return common.NewPipelineExecutor(
			ntErr,
			func(ctx context.Context) error {
				actionModel, err := sar.readAction(sar.Step, actionDir, remoteAction.Path, remoteReader(ctx), ioutil.WriteFile)
				sar.action = actionModel
				log.Debugf("Read action %v from '%s'", sar.action, "Unknown")
				return err
			},
			sar.runAction(sar, actionDir, remoteAction.Path, remoteAction.Repo, remoteAction.Ref, false),
		)(ctx)
	})
}

func (sar *stepActionRemote) post() common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
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

func (sar *stepActionRemote) getActionModel() *model.Action {
	return sar.action
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
