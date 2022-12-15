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
	planner, err := model.NewWorkflowPlanner(path.Join(directory, rc.Run.Job().Uses), true)
	if err != nil {
		return common.NewErrorExecutor(err)
	}

	plan := planner.PlanEvent("workflow_call")

	runner, err := NewReusableWorkflowRunner(rc)
	if err != nil {
		return common.NewErrorExecutor(err)
	}

	return runner.NewPlanExecutor(plan)
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
