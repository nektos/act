package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"

	"github.com/nektos/act/pkg/model"
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

type TestJobFileInfo struct {
	workdir               string
	workflowPath          string
	eventName             string
	errorMessage          string
	platforms             map[string]string
	containerArchitecture string
}

func runTestJobFile(ctx context.Context, t *testing.T, tjfi TestJobFileInfo) {
	t.Run(tjfi.workflowPath, func(t *testing.T) {
		workdir, err := filepath.Abs(tjfi.workdir)
		assert.NilError(t, err, workdir)
		fullWorkflowPath := filepath.Join(workdir, tjfi.workflowPath)
		runnerConfig := &Config{
			Workdir:               workdir,
			BindWorkdir:           true,
			EventName:             tjfi.eventName,
			Platforms:             tjfi.platforms,
			ReuseContainers:       false,
			ContainerArchitecture: tjfi.containerArchitecture,
		}
		runner, err := New(runnerConfig)
		assert.NilError(t, err, tjfi.workflowPath)

		planner, err := model.NewWorkflowPlanner(fullWorkflowPath)
		assert.NilError(t, err, fullWorkflowPath)

		plan := planner.PlanEvent(tjfi.eventName)

		err = runner.NewPlanExecutor(plan)(ctx)
		if tjfi.errorMessage == "" {
			assert.NilError(t, err, fullWorkflowPath)
		} else {
			assert.ErrorContains(t, err, tjfi.errorMessage)
		}
	})
}

func TestRunEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	platforms := map[string]string{
		"ubuntu-latest": "node:12.20.1-buster-slim",
	}
	tables := []TestJobFileInfo{
		// {"testdata", "powershell", "push", "", platforms}, // Powershell is not available on default act test runner (yet) but preserving here for posterity
		{"testdata", "basic", "push", "", platforms, "linux/amd64"},
		{"testdata", "fail", "push", "exit with `FAILURE`: 1", platforms, "linux/amd64"},
		{"testdata", "runs-on", "push", "", platforms, "linux/amd64"},
		{"testdata", "job-container", "push", "", platforms, "linux/amd64"},
		{"testdata", "job-container-non-root", "push", "", platforms, "linux/amd64"},
		{"testdata", "uses-docker-url", "push", "", platforms, "linux/amd64"},
		{"testdata", "remote-action-docker", "push", "", platforms, "linux/amd64"},
		{"testdata", "remote-action-js", "push", "", platforms, "linux/amd64"},
		{"testdata", "local-action-docker-url", "push", "", platforms, "linux/amd64"},
		{"testdata", "local-action-dockerfile", "push", "", platforms, "linux/amd64"},
		{"testdata", "local-action-js", "push", "", platforms, "linux/amd64"},
		{"testdata", "matrix", "push", "", platforms, "linux/amd64"},
		{"testdata", "matrix-include-exclude", "push", "", platforms, "linux/amd64"},
		{"testdata", "commands", "push", "", platforms, "linux/amd64"},
		{"testdata", "workdir", "push", "", platforms, "linux/amd64"},
		// {"testdata", "issue-228", "push", "", platforms, "linux/amd64"}, // TODO [igni]: Remove this once everything passes
		{"testdata", "defaults-run", "push", "", platforms, "linux/amd64"},
		{"testdata", "uses-composite", "push", "", platforms, "linux/amd64"},
		{"testdata", "issue-597", "push", "", platforms, "linux/amd64"},
		// linux/arm64
		{"testdata", "basic", "push", "", platforms, "linux/arm64"},
		{"testdata", "fail", "push", "exit with `FAILURE`: 1", platforms, "linux/arm64"},
		{"testdata", "runs-on", "push", "", platforms, "linux/arm64"},
		{"testdata", "job-container", "push", "", platforms, "linux/arm64"},
		{"testdata", "job-container-non-root", "push", "", platforms, "linux/arm64"},
		{"testdata", "uses-docker-url", "push", "", platforms, "linux/arm64"},
		{"testdata", "remote-action-docker", "push", "", platforms, "linux/arm64"},
		{"testdata", "remote-action-js", "push", "", platforms, "linux/arm64"},
		{"testdata", "local-action-docker-url", "push", "", platforms, "linux/arm64"},
		{"testdata", "local-action-dockerfile", "push", "", platforms, "linux/arm64"},
		{"testdata", "local-action-js", "push", "", platforms, "linux/arm64"},
		{"testdata", "matrix", "push", "", platforms, "linux/arm64"},
		{"testdata", "matrix-include-exclude", "push", "", platforms, "linux/arm64"},
		{"testdata", "commands", "push", "", platforms, "linux/arm64"},
		{"testdata", "workdir", "push", "", platforms, "linux/arm64"},
		// {"testdata", "issue-228", "push", "", platforms, "linux/arm64"}, // TODO [igni]: Remove this once everything passes
		{"testdata", "defaults-run", "push", "", platforms, "linux/arm64"},
		{"testdata", "uses-composite", "push", "", platforms, "linux/arm64"},
		{"testdata", "issue-597", "push", "", platforms, "linux/arm64"},
	}
	log.SetLevel(log.DebugLevel)

	ctx := context.Background()

	for _, table := range tables {
		runTestJobFile(ctx, t, table)
	}
}

func TestRunEventSecrets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	log.SetLevel(log.DebugLevel)
	ctx := context.Background()

	platforms := map[string]string{
		"ubuntu-latest": "node:12.20.1-buster-slim",
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
		"ubuntu-latest": "node:12.20.1-buster-slim",
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
