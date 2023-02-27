package runner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sync"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/common/git"
	"github.com/nektos/act/pkg/model"
)

func newLocalReusableWorkflowExecutor(rc *RunContext) common.Executor {
	return newReusableWorkflowExecutor(rc, rc.Config.Workdir, rc.Run.Job().Uses)
}

func newRemoteReusableWorkflowExecutor(rc *RunContext) common.Executor {
	uses := rc.Run.Job().Uses

	remoteReusableWorkflow := newRemoteReusableWorkflow(uses)
	if remoteReusableWorkflow == nil {
		return common.NewErrorExecutor(fmt.Errorf("expected format {owner}/{repo}/.github/workflows/{filename}@{ref}. Actual '%s' Input string was not in a correct format", uses))
	}
	remoteReusableWorkflow.URL = rc.Config.GitHubInstance

	workflowDir := fmt.Sprintf("%s/%s", rc.ActionCacheDir(), safeFilename(uses))

	return common.NewPipelineExecutor(
		newMutexExecutor(cloneIfRequired(rc, *remoteReusableWorkflow, workflowDir)),
		newReusableWorkflowExecutor(rc, workflowDir, fmt.Sprintf("./.github/workflows/%s", remoteReusableWorkflow.Filename)),
	)
}

var (
	executorLock sync.Mutex
)

func newMutexExecutor(executor common.Executor) common.Executor {
	return func(ctx context.Context) error {
		executorLock.Lock()
		defer executorLock.Unlock()

		return executor(ctx)
	}
}

func cloneIfRequired(rc *RunContext, remoteReusableWorkflow remoteReusableWorkflow, targetDirectory string) common.Executor {
	return common.NewConditionalExecutor(
		func(ctx context.Context) bool {
			_, err := os.Stat(targetDirectory)
			notExists := errors.Is(err, fs.ErrNotExist)
			return notExists
		},
		git.NewGitCloneExecutor(git.NewGitCloneExecutorInput{
			URL:   remoteReusableWorkflow.CloneURL(),
			Ref:   remoteReusableWorkflow.Ref,
			Dir:   targetDirectory,
			Token: rc.Config.Token,
		}),
		nil,
	)
}

func newReusableWorkflowExecutor(rc *RunContext, directory string, workflow string) common.Executor {
	return func(ctx context.Context) error {
		planner, err := model.NewWorkflowPlanner(path.Join(directory, workflow), true)
		if err != nil {
			return err
		}

		plan, err := planner.PlanEvent("workflow_call")
		if err != nil {
			return err
		}

		runner, err := NewReusableWorkflowRunner(rc)
		if err != nil {
			return err
		}

		return runner.NewPlanExecutor(plan)(ctx)
	}
}

func NewReusableWorkflowRunner(rc *RunContext) (Runner, error) {
	runner := &runnerImpl{
		config:    rc.Config,
		eventJSON: rc.EventJSON,
		caller: &caller{
			runContext: rc,
		},
	}

	return runner.configure()
}

type remoteReusableWorkflow struct {
	URL      string
	Org      string
	Repo     string
	Filename string
	Ref      string
}

func (r *remoteReusableWorkflow) CloneURL() string {
	return fmt.Sprintf("https://%s/%s/%s", r.URL, r.Org, r.Repo)
}

func newRemoteReusableWorkflow(uses string) *remoteReusableWorkflow {
	// GitHub docs:
	// https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_iduses
	r := regexp.MustCompile(`^([^/]+)/([^/]+)/.github/workflows/([^@]+)@(.*)$`)
	matches := r.FindStringSubmatch(uses)
	if len(matches) != 5 {
		return nil
	}
	return &remoteReusableWorkflow{
		Org:      matches[1],
		Repo:     matches[2],
		Filename: matches[3],
		Ref:      matches[4],
		URL:      "github.com",
	}
}
