package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"github.com/nektos/act/pkg/model"

	log "github.com/sirupsen/logrus"
	"gotest.tools/assert"
)

func TestGraphEvent(t *testing.T) {
	planner, err := model.NewWorkflowPlanner("testdata/basic")
	assert.NilError(t, err)

	plan := planner.PlanEvent("push")
	assert.NilError(t, err)
	assert.Equal(t, len(plan.Stages), 3, "stages")
	assert.Equal(t, len(plan.Stages[0].Runs), 1, "stage0.runs")
	assert.Equal(t, len(plan.Stages[1].Runs), 1, "stage1.runs")
	assert.Equal(t, len(plan.Stages[2].Runs), 1, "stage2.runs")
	assert.Equal(t, plan.Stages[0].Runs[0].JobID, "check", "jobid")
	assert.Equal(t, plan.Stages[1].Runs[0].JobID, "build", "jobid")
	assert.Equal(t, plan.Stages[2].Runs[0].JobID, "test", "jobid")

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
		{"job-container-non-root", "push", ""},
		{"uses-docker-url", "push", ""},
		{"remote-action-docker", "push", ""},
		{"remote-action-js", "push", ""},
		{"local-action-docker-url", "push", ""},
		{"local-action-dockerfile", "push", ""},
		{"matrix", "push", ""},
		{"commands", "push", ""},
		{"workdir", "push", ""},
		{"issue-228", "push", ""}, // TODO [igni]: Remove this once everything passes
		{"defaults-run", "push", ""},
	}
	log.SetLevel(log.DebugLevel)

	ctx := context.Background()

	for _, table := range tables {
		table := table
		t.Run(table.workflowPath, func(t *testing.T) {
			platforms := map[string]string{
				"ubuntu-latest": "node:12.6-buster-slim",
			}

			workdir, err := filepath.Abs("testdata")
			assert.NilError(t, err, table.workflowPath)
			runnerConfig := &Config{
				Workdir:         workdir,
				EventName:       table.eventName,
				Platforms:       platforms,
				ReuseContainers: false,
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

func TestRunEventSecrets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	log.SetLevel(log.DebugLevel)
	ctx := context.Background()

	platforms := map[string]string{
		"ubuntu-latest": "node:12.6-buster-slim",
	}

	workflowPath := "secrets"
	eventName := "push"

	workdir, err := filepath.Abs("testdata")
	assert.NilError(t, err, workflowPath)

	env, _ := godotenv.Read(filepath.Join(workdir, workflowPath, ".env"))
	secrets, _ := godotenv.Read(filepath.Join(workdir, workflowPath, ".secrets"))

	runnerConfig := &Config{
		Workdir:         workdir,
		EventName:       eventName,
		Platforms:       platforms,
		ReuseContainers: false,
		Secrets:         secrets,
		Env:             env,
	}
	runner, err := New(runnerConfig)
	assert.NilError(t, err, workflowPath)

	planner, err := model.NewWorkflowPlanner(fmt.Sprintf("testdata/%s", workflowPath))
	assert.NilError(t, err, workflowPath)

	plan := planner.PlanEvent(eventName)

	err = runner.NewPlanExecutor(plan)(ctx)
	assert.NilError(t, err, workflowPath)
}

func TestRunEventPullRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	log.SetLevel(log.DebugLevel)
	ctx := context.Background()

	platforms := map[string]string{
		"ubuntu-latest": "node:12.6-buster-slim",
	}

	workflowPath := "pull-request"
	eventName := "pull_request"

	workdir, err := filepath.Abs("testdata")
	assert.NilError(t, err, workflowPath)

	runnerConfig := &Config{
		Workdir:         workdir,
		EventName:       eventName,
		EventPath:       filepath.Join(workdir, workflowPath, "event.json"),
		Platforms:       platforms,
		ReuseContainers: false,
	}
	runner, err := New(runnerConfig)
	assert.NilError(t, err, workflowPath)

	planner, err := model.NewWorkflowPlanner(fmt.Sprintf("testdata/%s", workflowPath))
	assert.NilError(t, err, workflowPath)

	plan := planner.PlanEvent(eventName)

	err = runner.NewPlanExecutor(plan)(ctx)
	assert.NilError(t, err, workflowPath)
}
