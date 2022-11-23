package runner

import (
	"fmt"
	"path"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

func newLocalReusableWorkflowExecutor(rc *RunContext) common.Executor {
	return newReusableWorkflowExecutor(rc, rc.Config.Workdir)
}

func newRemoteReusableWorkflowExecutor(rc *RunContext) common.Executor {
	return common.NewErrorExecutor(fmt.Errorf("remote reusable workflows are currently not supported (see https://github.com/nektos/act/issues/826 for updates)"))
}

func newReusableWorkflowExecutor(rc *RunContext, directory string) common.Executor {
	job := rc.Run.Job()

	planner, err := model.NewWorkflowPlanner(path.Join(directory, job.Uses), true)
	if err != nil {
		return common.NewErrorExecutor(err)
	}

	plan := planner.PlanEvent("workflow_call")

	runner, err := NewReusableWorkflowRunner(rc.Config, job)
	if err != nil {
		return common.NewErrorExecutor(err)
	}

	return runner.NewPlanExecutor(plan)
}

func NewReusableWorkflowRunner(runnerConfig *Config, job *model.Job) (Runner, error) {
	runner := &runnerImpl{
		config: runnerConfig,
		caller: &caller{
			job: job,
		},
	}

	return runner.configure()
}
