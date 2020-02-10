package runner

import (
	"context"
	"fmt"
	"testing"

	"github.com/nektos/act/pkg/model"

	log "github.com/sirupsen/logrus"
	"gotest.tools/assert"
)

func TestGraphEvent(t *testing.T) {
	planner, err := model.NewWorkflowPlanner("testdata/basic")
	assert.NilError(t, err)

	plan := planner.PlanEvent("push")
	assert.NilError(t, err)
	assert.Equal(t, len(plan.Stages), 2, "stages")
	assert.Equal(t, len(plan.Stages[0].Runs), 1, "stage0.runs")
	assert.Equal(t, len(plan.Stages[1].Runs), 1, "stage1.runs")
	assert.Equal(t, plan.Stages[0].Runs[0].JobID, "build", "jobid")
	assert.Equal(t, plan.Stages[1].Runs[0].JobID, "test", "jobid")

	plan = planner.PlanEvent("release")
	assert.Equal(t, len(plan.Stages), 0, "stages")
}

func TestRunEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tables := []struct {
		workflowPath string
		eventName    string
		errorMessage string
	}{
		{"basic", "push", ""},
		{"fail", "push", "exit with `FAILURE`: 1"},
		{"runs-on", "push", ""},
		{"job-container", "push", ""},
		{"uses-docker-url", "push", ""},
		{"remote-action-docker", "push", ""},
		{"remote-action-js", "push", ""},
		{"local-action-docker-url", "push", ""},
		{"local-action-dockerfile", "push", ""},
	}
	log.SetLevel(log.DebugLevel)

	ctx := context.Background()

	for _, table := range tables {
		table := table
		t.Run(table.workflowPath, func(t *testing.T) {
			runnerConfig := &Config{
				Workdir:   "testdata",
				EventName: table.eventName,
			}
			runner, err := New(runnerConfig)
			assert.NilError(t, err, table.workflowPath)

			planner, err := model.NewWorkflowPlanner(fmt.Sprintf("testdata/%s", table.workflowPath))
			assert.NilError(t, err, table.workflowPath)

			plan := planner.PlanEvent(table.eventName)

			err = runner.NewPlanExecutor(plan)(ctx)
			if table.errorMessage == "" {
				assert.NilError(t, err, table.workflowPath)
			} else {
				assert.ErrorContains(t, err, table.errorMessage)
			}
		})
	}
}
