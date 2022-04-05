package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"

	"github.com/nektos/act/pkg/model"
)

var (
	baseImage = "node:16-buster-slim"
	platforms map[string]string
	logLevel  = log.DebugLevel
	workdir   = "testdata"
)

func init() {
	if p := os.Getenv("ACT_TEST_IMAGE"); p != "" {
		baseImage = p
	}

	platforms = map[string]string{
		"ubuntu-latest": baseImage,
	}

	if l := os.Getenv("ACT_TEST_LOG_LEVEL"); l != "" {
		if lvl, err := log.ParseLevel(l); err == nil {
			logLevel = lvl
		}
	}

	if wd, err := filepath.Abs(workdir); err == nil {
		workdir = wd
	}
}

func TestGraphEvent(t *testing.T) {
	planner, err := model.NewWorkflowPlanner("testdata/basic", true)
	assert.Nil(t, err)

	plan := planner.PlanEvent("push")
	assert.Nil(t, err)
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
	workdir      string
	workflowPath string
	eventName    string
	errorMessage string
	platforms    map[string]string
}

func (j *TestJobFileInfo) runTest(ctx context.Context, t *testing.T, cfg *Config) {
	log.SetLevel(logLevel)

	workdir, err := filepath.Abs(j.workdir)
	assert.Nil(t, err, workdir)

	fullWorkflowPath := filepath.Join(workdir, j.workflowPath)
	runnerConfig := &Config{
		Workdir:               workdir,
		BindWorkdir:           false,
		EventName:             j.eventName,
		EventPath:             cfg.EventPath,
		Platforms:             j.platforms,
		ReuseContainers:       false,
		Env:                   cfg.Env,
		Secrets:               cfg.Secrets,
		GitHubInstance:        "github.com",
		ContainerArchitecture: cfg.ContainerArchitecture,
	}

	runner, err := New(runnerConfig)
	assert.Nil(t, err, j.workflowPath)

	planner, err := model.NewWorkflowPlanner(fullWorkflowPath, true)
	assert.Nil(t, err, fullWorkflowPath)

	plan := planner.PlanEvent(j.eventName)

	err = runner.NewPlanExecutor(plan)(ctx)
	if j.errorMessage == "" {
		assert.Nil(t, err, fullWorkflowPath)
	} else {
		assert.Error(t, err, j.errorMessage)
	}
}

func runTestJobFile(ctx context.Context, t *testing.T, j TestJobFileInfo) {
	t.Run(j.workflowPath, func(t *testing.T) {
		j.runTest(ctx, t, &Config{})
	})
}

func TestRunEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	tables := []TestJobFileInfo{
		// Shells
		{workdir, "shells/defaults", "push", "", platforms},
		// TODO: figure out why it fails
		// {workdir, "shells/custom", "push", "", map[string]string{"ubuntu-latest": "ghcr.io/justingrote/act-pwsh:latest"}, }, // custom image with pwsh
		{workdir, "shells/pwsh", "push", "", map[string]string{"ubuntu-latest": "ghcr.io/justingrote/act-pwsh:latest"}}, // custom image with pwsh
		{workdir, "shells/bash", "push", "", platforms},
		{workdir, "shells/python", "push", "", map[string]string{"ubuntu-latest": "node:16-buster"}}, // slim doesn't have python
		{workdir, "shells/sh", "push", "", platforms},

		// Local action
		{workdir, "local-action-docker-url", "push", "", platforms},
		{workdir, "local-action-dockerfile", "push", "", platforms},
		{workdir, "local-action-via-composite-dockerfile", "push", "", platforms},
		{workdir, "local-action-js", "push", "", platforms},

		// Uses
		{workdir, "uses-composite", "push", "", platforms},
		{workdir, "uses-composite-with-error", "push", "Job 'failing-composite-action' failed", platforms},
		{workdir, "uses-nested-composite", "push", "", platforms},
		{workdir, "uses-workflow", "push", "reusable workflows are currently not supported (see https://github.com/nektos/act/issues/826 for updates)", platforms},
		{workdir, "uses-docker-url", "push", "", platforms},

		// Eval
		{workdir, "evalmatrix", "push", "", platforms},
		{workdir, "evalmatrixneeds", "push", "", platforms},
		{workdir, "evalmatrixneeds2", "push", "", platforms},
		{workdir, "evalmatrix-merge-map", "push", "", platforms},
		{workdir, "evalmatrix-merge-array", "push", "", platforms},

		{workdir, "basic", "push", "", platforms},
		{workdir, "fail", "push", "exit with `FAILURE`: 1", platforms},
		{workdir, "runs-on", "push", "", platforms},
		{workdir, "checkout", "push", "", platforms},
		{workdir, "job-container", "push", "", platforms},
		{workdir, "job-container-non-root", "push", "", platforms},
		{workdir, "container-hostname", "push", "", platforms},
		{workdir, "remote-action-docker", "push", "", platforms},
		{workdir, "remote-action-js", "push", "", platforms},
		{workdir, "matrix", "push", "", platforms},
		{workdir, "matrix-include-exclude", "push", "", platforms},
		{workdir, "commands", "push", "", platforms},
		{workdir, "workdir", "push", "", platforms},
		{workdir, "defaults-run", "push", "", platforms},
		{workdir, "composite-fail-with-output", "push", "", platforms},
		{workdir, "issue-597", "push", "", platforms},
		{workdir, "issue-598", "push", "", platforms},
		{workdir, "if-env-act", "push", "", platforms},
		{workdir, "env-and-path", "push", "", platforms},
		{workdir, "non-existent-action", "push", "Job 'nopanic' failed", platforms},
		{workdir, "outputs", "push", "", platforms},
		{workdir, "steps-context/conclusion", "push", "", platforms},
		{workdir, "steps-context/outcome", "push", "", platforms},
		{workdir, "job-status-check", "push", "job 'fail' failed", platforms},
		{workdir, "if-expressions", "push", "Job 'mytest' failed", platforms},
		{"../model/testdata", "strategy", "push", "", platforms}, // TODO: move all testdata into pkg so we can validate it with planner and runner
		// {"testdata", "issue-228", "push", "", platforms, }, // TODO [igni]: Remove this once everything passes
		{"../model/testdata", "container-volumes", "push", "", platforms},
	}

	for _, table := range tables {
		runTestJobFile(ctx, t, table)
	}
}

func TestRunDifferentArchitecture(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tjfi := TestJobFileInfo{
		workdir:      workdir,
		workflowPath: "basic",
		eventName:    "push",
		errorMessage: "",
		platforms:    platforms,
	}

	tjfi.runTest(context.Background(), t, &Config{ContainerArchitecture: "linux/arm64"})
}

func TestRunEventSecrets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	workflowPath := "secrets"

	tjfi := TestJobFileInfo{
		workdir:      workdir,
		workflowPath: workflowPath,
		eventName:    "push",
		errorMessage: "",
		platforms:    platforms,
	}

	env, err := godotenv.Read(filepath.Join(workdir, workflowPath, ".env"))
	assert.NoError(t, err, "Failed to read .env")
	secrets, _ := godotenv.Read(filepath.Join(workdir, workflowPath, ".secrets"))
	assert.NoError(t, err, "Failed to read .secrets")

	tjfi.runTest(context.Background(), t, &Config{Secrets: secrets, Env: env})
}

func TestRunEventPullRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	workflowPath := "pull-request"

	tjfi := TestJobFileInfo{
		workdir:      workdir,
		workflowPath: workflowPath,
		eventName:    "pull_request",
		errorMessage: "",
		platforms:    platforms,
	}

	tjfi.runTest(context.Background(), t, &Config{EventPath: filepath.Join(workdir, workflowPath, "event.json")})
}

func TestContainerPath(t *testing.T) {
	type containerPathJob struct {
		destinationPath string
		sourcePath      string
		workDir         string
	}

	if runtime.GOOS == "windows" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Error(err)
		}

		rootDrive := os.Getenv("SystemDrive")
		rootDriveLetter := strings.ReplaceAll(strings.ToLower(rootDrive), `:`, "")
		for _, v := range []containerPathJob{
			{"/mnt/c/Users/act/go/src/github.com/nektos/act", "C:\\Users\\act\\go\\src\\github.com\\nektos\\act\\", ""},
			{"/mnt/f/work/dir", `F:\work\dir`, ""},
			{"/mnt/c/windows/to/unix", "windows\\to\\unix", fmt.Sprintf("%s\\", rootDrive)},
			{fmt.Sprintf("/mnt/%v/act", rootDriveLetter), "act", fmt.Sprintf("%s\\", rootDrive)},
		} {
			if v.workDir != "" {
				if err := os.Chdir(v.workDir); err != nil {
					log.Error(err)
					t.Fail()
				}
			}

			runnerConfig := &Config{
				Workdir: v.sourcePath,
			}

			assert.Equal(t, v.destinationPath, runnerConfig.containerPath(runnerConfig.Workdir))
		}

		if err := os.Chdir(cwd); err != nil {
			log.Error(err)
		}
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			log.Error(err)
		}
		for _, v := range []containerPathJob{
			{"/home/act/go/src/github.com/nektos/act", "/home/act/go/src/github.com/nektos/act", ""},
			{"/home/act", `/home/act/`, ""},
			{cwd, ".", ""},
		} {
			runnerConfig := &Config{
				Workdir: v.sourcePath,
			}

			assert.Equal(t, v.destinationPath, runnerConfig.containerPath(runnerConfig.Workdir))
		}
	}
}
