package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nektos/act/pkg/model"
)

func TestExpandParallelStepGroups(t *testing.T) {
	workflow, err := model.ReadWorkflow(strings.NewReader(`
name: parallel
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo before
      - parallel:
          - name: Build a
            run: echo a
          - name: Build b
            id: build-b
            run: echo b
      - run: echo after
`), false)
	require.NoError(t, err)

	steps, err := expandParallelStepGroups(workflow.GetJob("test").Steps)
	require.NoError(t, err)
	require.Len(t, steps, 5)

	assert.Equal(t, "echo before", steps[0].Run)

	assert.Equal(t, "echo a", steps[1].Run)
	assert.True(t, steps[1].IsBackground())
	assert.Equal(t, "1", steps[1].ID)

	assert.Equal(t, "echo b", steps[2].Run)
	assert.True(t, steps[2].IsBackground())
	assert.Equal(t, "build-b", steps[2].ID)

	assert.Equal(t, model.StepTypeWait, steps[3].Type())
	assert.Equal(t, []string{"1", "build-b"}, steps[3].Wait())

	assert.Equal(t, "echo after", steps[4].Run)
}

func runBackgroundStepsWorkflow(t *testing.T, workflowPath string, markerDir string) (*model.Plan, error) {
	t.Helper()

	log.SetLevel(logLevel)

	absWorkdir, err := filepath.Abs(workdir)
	require.NoError(t, err)

	config := &Config{
		Workdir:        absWorkdir,
		EventName:      "push",
		Platforms:      map[string]string{"self-hosted": "-self-hosted"},
		GitHubInstance: "github.com",
		Env:            map[string]string{"MARKER_DIR": markerDir},
	}
	runner, err := New(config)
	require.NoError(t, err)

	planner, err := model.NewWorkflowPlanner(filepath.Join(absWorkdir, workflowPath), true, false)
	require.NoError(t, err)
	plan, err := planner.PlanEvent("push")
	require.NoError(t, err)

	return plan, runner.NewPlanExecutor(plan)(context.Background())
}

func backgroundStepsWorkflowName(name string) string {
	if runtime.GOOS == "windows" {
		return name + "-windows"
	}
	return name
}

func markerModTime(t *testing.T, dir string, name string) time.Time {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, name))
	require.NoError(t, err, "marker file %s should exist", name)
	return info.ModTime()
}

func TestRunBackgroundSteps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	markerDir := filepath.ToSlash(t.TempDir())
	plan, err := runBackgroundStepsWorkflow(t, backgroundStepsWorkflowName("background-steps"), markerDir)
	assert.NoError(t, err)

	for _, stage := range plan.Stages {
		for _, run := range stage.Runs {
			assert.Equal(t, "success", run.Job().Result, "job %s should have succeeded", run.String())
		}
	}

	// the next step ran while the background step was still sleeping
	workStart := markerModTime(t, markerDir, "work-start")
	serviceEnd := markerModTime(t, markerDir, "service-end")
	assert.True(t, workStart.Before(serviceEnd),
		"the step after a background step must not wait for it (work started %v, service ended %v)", workStart, serviceEnd)

	// both steps of the parallel group overlapped and the following step ran
	// after the implicit wait for the whole group
	aStart := markerModTime(t, markerDir, "a-start")
	aEnd := markerModTime(t, markerDir, "a-end")
	bStart := markerModTime(t, markerDir, "b-start")
	bEnd := markerModTime(t, markerDir, "b-end")
	assert.True(t, aStart.Before(bEnd) && bStart.Before(aEnd),
		"steps of a parallel group must run concurrently (a: %v - %v, b: %v - %v)", aStart, aEnd, bStart, bEnd)
	afterParallel := markerModTime(t, markerDir, "after-parallel")
	assert.False(t, afterParallel.Before(aEnd), "the step after a parallel group must run after all steps of the group")
	assert.False(t, afterParallel.Before(bEnd), "the step after a parallel group must run after all steps of the group")

	// the quick background step completed even though the monitor was cancelled
	assert.FileExists(t, filepath.Join(markerDir, "quick-done"))
}

func TestRunBackgroundStepFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	plan, err := runBackgroundStepsWorkflow(t, backgroundStepsWorkflowName("background-steps-fail"), filepath.ToSlash(t.TempDir()))
	assert.ErrorContains(t, err, "' failed")

	results := map[string]string{}
	for _, stage := range plan.Stages {
		for _, run := range stage.Runs {
			results[run.JobID] = run.Job().Result
		}
	}
	assert.Equal(t, "failure", results["fail-at-wait"], "a failing background step must fail the job at the wait step")
	assert.Equal(t, "success", results["continue-on-error"], "continue-on-error on a background step must swallow its failure")
	assert.Equal(t, "failure", results["duplicate-id"], "duplicate background step ids must fail the job instead of hiding a step")
	assert.Equal(t, "failure", results["cancel-after-failure"], "cancelling an already failed background step must not hide its failure from a later wait")
}
