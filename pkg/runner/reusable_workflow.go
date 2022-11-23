package runner

import (
	"path"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

func newLocalReusableWorkflowExecutor(rc *RunContext) common.Executor {
	job := rc.Run.Job()

	planner, err := model.NewWorkflowPlanner(path.Join(rc.Config.Workdir, job.Uses), true)
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
