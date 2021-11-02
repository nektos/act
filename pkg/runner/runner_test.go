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
	logTest "github.com/sirupsen/logrus/hooks/test"
	assert "github.com/stretchr/testify/assert"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

var baseImage = "node:12-buster-slim"

func init() {
	if p := os.Getenv("ACT_TEST_IMAGE"); p != "" {
		baseImage = p
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
		assert.Nil(t, err, workdir)
		fullWorkflowPath := filepath.Join(workdir, tjfi.workflowPath)
		runnerConfig := &Config{
			Workdir:               workdir,
			BindWorkdir:           false,
			EventName:             tjfi.eventName,
			Platforms:             tjfi.platforms,
			ReuseContainers:       false,
			ContainerArchitecture: tjfi.containerArchitecture,
			GitHubInstance:        "github.com",
		}

		runner, err := New(runnerConfig)
		assert.Nil(t, err, tjfi.workflowPath)

		planner, err := model.NewWorkflowPlanner(fullWorkflowPath, true)
		assert.Nil(t, err, fullWorkflowPath)

		plan := planner.PlanEvent(tjfi.eventName)

		err = runner.NewPlanExecutor(plan)(ctx)
		if tjfi.errorMessage == "" {
			assert.Nil(t, err, fullWorkflowPath)
		} else {
			assert.Error(t, err, tjfi.errorMessage)
		}
	})
}

func TestRunEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	platforms := map[string]string{
		"ubuntu-latest": baseImage,
	}

	tables := []TestJobFileInfo{
		{"testdata", "basic", "push", "", platforms, ""},
		{"testdata", "fail", "push", "exit with `FAILURE`: 1", platforms, ""},
		{"testdata", "runs-on", "push", "", platforms, ""},
		{"testdata", "checkout", "push", "", platforms, ""},
		{"testdata", "shells/defaults", "push", "", platforms, ""},
		{"testdata", "shells/pwsh", "push", "", map[string]string{"ubuntu-latest": "ghcr.io/justingrote/act-pwsh:latest"}, ""}, // custom image with pwsh
		{"testdata", "shells/bash", "push", "", platforms, ""},
		{"testdata", "shells/python", "push", "", map[string]string{"ubuntu-latest": "node:12-buster"}, ""}, // slim doesn't have python
		{"testdata", "shells/sh", "push", "", platforms, ""},
		{"testdata", "job-container", "push", "", platforms, ""},
		{"testdata", "job-container-non-root", "push", "", platforms, ""},
		{"testdata", "container-hostname", "push", "", platforms, ""},
		{"testdata", "uses-docker-url", "push", "", platforms, ""},
		{"testdata", "remote-action-docker", "push", "", platforms, ""},
		{"testdata", "remote-action-js", "push", "", platforms, ""},
		{"testdata", "local-action-docker-url", "push", "", platforms, ""},
		{"testdata", "local-action-dockerfile", "push", "", platforms, ""},
		{"testdata", "local-action-js", "push", "", platforms, ""},
		{"testdata", "matrix", "push", "", platforms, ""},
		{"testdata", "matrix-include-exclude", "push", "", platforms, ""},
		{"testdata", "commands", "push", "", platforms, ""},
		{"testdata", "workdir", "push", "", platforms, ""},
		{"testdata", "defaults-run", "push", "", platforms, ""},
		{"testdata", "uses-composite", "push", "", platforms, ""},
		{"testdata", "issue-597", "push", "", platforms, ""},
		{"testdata", "issue-598", "push", "", platforms, ""},
		{"testdata", "env-and-path", "push", "", platforms, ""},
		{"testdata", "outputs", "push", "", platforms, ""},
		{"testdata", "steps-context/conclusion", "push", "", platforms, ""},
		{"testdata", "steps-context/outcome", "push", "", platforms, ""},
		{"../model/testdata", "strategy", "push", "", platforms, ""}, // TODO: move all testdata into pkg so we can validate it with planner and runner
		// {"testdata", "issue-228", "push", "", platforms, ""}, // TODO [igni]: Remove this once everything passes

		// single test for different architecture: linux/arm64
		{"testdata", "basic", "push", "", platforms, "linux/arm64"},
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
		"ubuntu-latest": baseImage,
	}

	workflowPath := "secrets"
	eventName := "push"

	workdir, err := filepath.Abs("testdata")
	assert.Nil(t, err, workflowPath)

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
	assert.Nil(t, err, workflowPath)

	planner, err := model.NewWorkflowPlanner(fmt.Sprintf("testdata/%s", workflowPath), true)
	assert.Nil(t, err, workflowPath)

	plan := planner.PlanEvent(eventName)

	err = runner.NewPlanExecutor(plan)(ctx)
	assert.Nil(t, err, workflowPath)
}

func TestRunEventPullRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	log.SetLevel(log.DebugLevel)
	ctx := context.Background()

	platforms := map[string]string{
		"ubuntu-latest": baseImage,
	}

	workflowPath := "pull-request"
	eventName := "pull_request"

	workdir, err := filepath.Abs("testdata")
	assert.Nil(t, err, workflowPath)

	runnerConfig := &Config{
		Workdir:         workdir,
		EventName:       eventName,
		EventPath:       filepath.Join(workdir, workflowPath, "event.json"),
		Platforms:       platforms,
		ReuseContainers: false,
	}
	runner, err := New(runnerConfig)
	assert.Nil(t, err, workflowPath)

	planner, err := model.NewWorkflowPlanner(fmt.Sprintf("testdata/%s", workflowPath), true)
	assert.Nil(t, err, workflowPath)

	plan := planner.PlanEvent(eventName)

	err = runner.NewPlanExecutor(plan)(ctx)
	assert.Nil(t, err, workflowPath)
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

func runWorkflowWithLogger(t *testing.T, workflowPath string) ([]string, []string, error) {
	logger, hook := logTest.NewNullLogger()
	globalHook := logTest.NewGlobal()

	if testing.Short() {
		t.Skip("skipping integration test")
	}

	platforms := map[string]string{
		"ubuntu-latest": baseImage,
	}
	workdir := "testdata"
	eventName := "push"

	log.SetLevel(log.DebugLevel)

	ctx := common.WithLogger(common.WithTestContext(context.Background()), logger)

	workdir, err := filepath.Abs(workdir)
	assert.Nil(t, err, workdir)

	fullWorkflowPath := filepath.Join(workdir, workflowPath)
	runnerConfig := &Config{
		Workdir:               workdir,
		BindWorkdir:           false,
		EventName:             eventName,
		Platforms:             platforms,
		ReuseContainers:       false,
		ContainerArchitecture: "",
		GitHubInstance:        "github.com",
	}

	runner, err := New(runnerConfig)
	assert.Nil(t, err, workflowPath)

	planner, err := model.NewWorkflowPlanner(fullWorkflowPath, true)
	assert.Nil(t, err, fullWorkflowPath)

	plan := planner.PlanEvent(eventName)

	err = runner.NewPlanExecutor(plan)(ctx)

	infoMessages := make([]string, 0)
	debugMessages := make([]string, 0)
	for _, entry := range hook.AllEntries() {
		if entry.Level == log.InfoLevel {
			infoMessages = append(infoMessages, entry.Message)
		} else if entry.Level == log.DebugLevel {
			debugMessages = append(debugMessages, entry.Message)
		}
	}

	for _, entry := range globalHook.AllEntries() {
		if entry.Level == log.DebugLevel {
			debugMessages = append(debugMessages, entry.Message)
		}
	}
	return infoMessages, debugMessages, err
}

func TestRemoteActionWithPreAndPostStep(t *testing.T) {
	infoMessages, debugMessages, err := runWorkflowWithLogger(t, "remote-action-with-pre-and-post-step")
	assert.Nil(t, err, "remote-action-with-pre-and-post-step")

	assert.Contains(t, infoMessages, "⭐  Run Pre xing/act/pkg/runner/testdata/actions/pre-post@pre-post")
	assert.Contains(t, debugMessages, `expression '${{ "post-if-never-true" == "true" }}' evaluated to 'false'`)
	assert.Contains(t, infoMessages, "⭐  Run Post xing/act/pkg/runner/testdata/actions/pre-post@pre-post")
	assert.Contains(t, debugMessages, `expression '${{ "post-if-never-true" == "true" }}' evaluated to 'false'`)
}

func TestRemoteActionWithMainSkipped(t *testing.T) {
	infoMessages, debugMessages, err := runWorkflowWithLogger(t, "remote-action-with-main-skipped")
	assert.Nil(t, err, "remote-action-with-main-skipped")

	assert.Contains(t, infoMessages, "⭐  Run Pre xing/act/pkg/runner/testdata/actions/pre-post@pre-post")
	assert.Contains(t, debugMessages, `Skipping step 'xing/act/pkg/runner/testdata/actions/pre-post@pre-post' due to 'false'`)
	assert.Contains(t, infoMessages, "⭐  Run Post xing/act/pkg/runner/testdata/actions/pre-post@pre-post")
}

func TestRemoteActionWithPreStepFailing(t *testing.T) {
	infoMessages, debugMessages, err := runWorkflowWithLogger(t, "remote-action-with-pre-step-failing")
	assert.Error(t, err, "remote-action-with-pre-step-failing")

	assert.Contains(t, infoMessages, "⭐  Run Pre xing/act/pkg/runner/testdata/actions/pre-post@pre-post")
	assert.Contains(t, debugMessages, "Error: Fail in pre step\n")
}

func TestRemoteActionWithPostStepFailing(t *testing.T) {
	infoMessages, debugMessages, err := runWorkflowWithLogger(t, "remote-action-with-post-step-failing")
	assert.Error(t, err, "remote-action-with-post-step-failing")

	assert.Contains(t, infoMessages, "⭐  Run Post xing/act/pkg/runner/testdata/actions/pre-post@pre-post")
	assert.Contains(t, debugMessages, "Error: Fail in post step\n")
}

func TestLocalActionWithPostStep(t *testing.T) {
	infoMessages, debugMessages, err := runWorkflowWithLogger(t, "local-action-with-post-step")
	assert.Nil(t, err, "local-action-with-post-step")

	assert.Contains(t, infoMessages, "⭐  Run local action with main")
	assert.Contains(t, debugMessages, "Skipping step 'local action with skipped main' due to 'false'")
	assert.Contains(t, infoMessages, "⭐  Run Post local action with main")
	assert.Contains(t, infoMessages, "⭐  Run Post local action with skipped main")
}
