package runner

import (
	"context"
	"path"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

func newLocalReusableWorkflowExecutor(rc *RunContext) common.Executor {
	return func(ctx context.Context) error {
		planner, err := model.NewWorkflowPlanner(path.Join(rc.Config.Workdir, rc.Run.Job().Uses), true)
		if err != nil {
			return err
		}

		plan := planner.PlanEvent("workflow_call")

		r, err := New(rc.Config)
		if err != nil {
			return err
		}

		executor := r.NewPlanExecutor(plan)

		return executor(ctx)
	}
}
