package runner

import (
	"archive/tar"
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
	if rc.caller != nil && rc.caller.reusableWorkflow != nil {
		localWorkflowPath := rc.Run.Job().Uses
		workflowFilename := path.Base(localWorkflowPath)
		reusableWorkflow := &remoteReusableWorkflow{
			Org:      rc.caller.reusableWorkflow.Org,
			Repo:     rc.caller.reusableWorkflow.Repo,
			Ref:      rc.caller.reusableWorkflow.Ref,
			URL:      rc.caller.reusableWorkflow.URL,
			Filename: workflowFilename,
		}
		if rc.Config.ActionCache != nil {
			return newActionCacheReusableWorkflowExecutor(rc, reusableWorkflow)
		}
		workflowDir := fmt.Sprintf("%s/%s", rc.ActionCacheDir(), safeFilename(reusableWorkflow.cacheKey()))
		return newReusableWorkflowExecutor(rc, workflowDir, fmt.Sprintf("./.github/workflows/%s", workflowFilename))
	}
	return newReusableWorkflowExecutor(rc, rc.Config.Workdir, rc.Run.Job().Uses)
}

func newRemoteReusableWorkflowExecutor(rc *RunContext) common.Executor {
	uses := rc.Run.Job().Uses

	remoteReusableWorkflow := newRemoteReusableWorkflow(uses)
	if remoteReusableWorkflow == nil {
		return common.NewErrorExecutor(fmt.Errorf("expected format {owner}/{repo}/.github/workflows/{filename}@{ref}. Actual '%s' Input string was not in a correct format", uses))
	}

	// uses with safe filename makes the target directory look something like this {owner}-{repo}-.github-workflows-{filename}@{ref}
	// instead we will just use {owner}-{repo}@{ref} as our target directory. This should also improve performance when we are using
	// multiple reusable workflows from the same repository and ref since for each workflow we won't have to clone it again
	workflowDir := fmt.Sprintf("%s/%s", rc.ActionCacheDir(), safeFilename(remoteReusableWorkflow.cacheKey()))

	if rc.Config.ActionCache != nil {
		return newActionCacheReusableWorkflowExecutor(rc, remoteReusableWorkflow)
	}

	return common.NewPipelineExecutor(
		newMutexExecutor(cloneIfRequired(rc, *remoteReusableWorkflow, workflowDir)),
		newReusableWorkflowExecutorWithRemote(rc, workflowDir, fmt.Sprintf("./.github/workflows/%s", remoteReusableWorkflow.Filename), remoteReusableWorkflow),
	)
}

func newActionCacheReusableWorkflowExecutor(rc *RunContext, remoteReusableWorkflow *remoteReusableWorkflow) common.Executor {
	return func(ctx context.Context) error {
		ghctx := rc.getGithubContext(ctx)
		remoteReusableWorkflow.URL = ghctx.ServerURL
		cacheKey := remoteReusableWorkflow.cacheKey()
		sha, err := rc.Config.ActionCache.Fetch(ctx, cacheKey, remoteReusableWorkflow.CloneURL(), remoteReusableWorkflow.Ref, ghctx.Token)
		if err != nil {
			return err
		}
		archive, err := rc.Config.ActionCache.GetTarArchive(ctx, cacheKey, sha, fmt.Sprintf(".github/workflows/%s", remoteReusableWorkflow.Filename))
		if err != nil {
			return err
		}
		defer archive.Close()
		treader := tar.NewReader(archive)
		if _, err = treader.Next(); err != nil {
			return err
		}
		planner, err := model.NewSingleWorkflowPlanner(remoteReusableWorkflow.Filename, treader)
		if err != nil {
			return err
		}
		plan, err := planner.PlanEvent("workflow_call")
		if err != nil {
			return err
		}

		runner, err := newReusableWorkflowRunner(rc, remoteReusableWorkflow)
		if err != nil {
			return err
		}

		return runner.NewPlanExecutor(plan)(ctx)
	}
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
		func(_ context.Context) bool {
			_, err := os.Stat(targetDirectory)
			notExists := errors.Is(err, fs.ErrNotExist)
			return notExists
		},
		func(ctx context.Context) error {
			remoteReusableWorkflow.URL = rc.getGithubContext(ctx).ServerURL
			return git.NewGitCloneExecutor(git.NewGitCloneExecutorInput{
				URL:         remoteReusableWorkflow.CloneURL(),
				Ref:         remoteReusableWorkflow.Ref,
				Dir:         targetDirectory,
				Token:       rc.Config.Token,
				OfflineMode: rc.Config.ActionOfflineMode,
			})(ctx)
		},
		nil,
	)
}

func newReusableWorkflowExecutor(rc *RunContext, directory string, workflow string) common.Executor {
	return newReusableWorkflowExecutorWithRemote(rc, directory, workflow, nil)
}

func newReusableWorkflowExecutorWithRemote(rc *RunContext, directory string, workflow string, remoteWorkflow *remoteReusableWorkflow) common.Executor {
	return func(ctx context.Context) error {
		planner, err := model.NewWorkflowPlanner(path.Join(directory, workflow), true, false)
		if err != nil {
			return err
		}

		plan, err := planner.PlanEvent("workflow_call")
		if err != nil {
			return err
		}

		runner, err := newReusableWorkflowRunner(rc, remoteWorkflow)
		if err != nil {
			return err
		}

		return runner.NewPlanExecutor(plan)(ctx)
	}
}

func NewReusableWorkflowRunner(rc *RunContext) (Runner, error) {
	return newReusableWorkflowRunner(rc, nil)
}

func newReusableWorkflowRunner(rc *RunContext, remoteWorkflow *remoteReusableWorkflow) (Runner, error) {
	runner := &runnerImpl{
		config:    rc.Config,
		eventJSON: rc.EventJSON,
		caller: &caller{
			runContext:       rc,
			reusableWorkflow: remoteWorkflow,
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
	return fmt.Sprintf("%s/%s/%s", r.URL, r.Org, r.Repo)
}

func (r *remoteReusableWorkflow) cacheKey() string {
	return fmt.Sprintf("%s/%s@%s", r.Org, r.Repo, r.Ref)
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
		URL:      "https://github.com",
	}
}
