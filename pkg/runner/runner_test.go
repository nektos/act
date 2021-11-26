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
	"github.com/stretchr/testify/mock"

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
		providers := &Providers{
			Action: NewActionProvider(),
		}

		runner, err := New(runnerConfig, providers)
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
		{"testdata", "local-action-js-with-post", "push", "", platforms, ""},
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
	providers := &Providers{
		Action: NewActionProvider(),
	}
	runner, err := New(runnerConfig, providers)
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
	providers := &Providers{
		Action: NewActionProvider(),
	}
	runner, err := New(runnerConfig, providers)
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

type actProviderMock struct {
	mock.Mock
	actProvider
}

func (m *actProviderMock) ExecuteNode12PostAction(ctx context.Context, sc *StepContext, containerActionDir string) error {
	name := sc.Action.Name
	m.MethodCalled(fmt.Sprintf("%s_ExecuteNode12PostAction",name), sc, containerActionDir, ctx)
	return m.actProvider.ExecuteNode12PostAction(ctx, sc, containerActionDir)
}

type TestJobPostStep struct {
	TestJobFileInfo
	called  map[string]bool // action name: Post called
	postCallOrder[] string // order of successful post called actions
}

func TestRunEventPostStep(t *testing.T) {
	tables := []TestJobPostStep{
		{called: map[string]bool{"node12-post-if-success": false},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/single-post-if-success-when-failure", errorMessage: "exit with `FAILURE`: 1"}},
		{called: map[string]bool{"node12-post-if-success": true},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/single-post-if-success-when-success", errorMessage: ""}},
		{called: map[string]bool{"node12-post-if-failure": true},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/single-post-if-failure-when-failure", errorMessage: "exit with `FAILURE`: 1"}},
		{called: map[string]bool{"node12-post-if-failure": false},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/single-post-if-failure-when-success", errorMessage: ""}},
		{called: map[string]bool{"node12-post-if-always": true},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/single-post-if-always-when-failure", errorMessage: "exit with `FAILURE`: 1"}},
		{called: map[string]bool{"node12-post-if-always": true},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/single-post-if-always-when-success", errorMessage: ""}},
		{called: map[string]bool{"node12-post-if-always": true, "node12-post-if-always-2": true, "node12-post-if-always-3": true},
			postCallOrder: []string{"node12-post-if-always-3", "node12-post-if-always-2", "node12-post-if-always"},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/multi-post-when-success", errorMessage: ""}},
		{called: map[string]bool{"node12-post-if-always": true, "node12-post-if-success": false, "node12-post-if-always-2": true},
			postCallOrder: []string{"node12-post-if-always-2", "node12-post-if-always"},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/multi-post-when-failed", errorMessage: "exit with `FAILURE`: 1"}},
		{called: map[string]bool{"node12-post-if-always": true, "node12-post-if-always-2": true, "node12-post-if-always-3": true},
			postCallOrder: []string{"node12-post-if-always-3", "node12-post-if-always-2", "node12-post-if-always"},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/multi-post-when-early-failure-and-continue-on-error", errorMessage: ""}},
		{called: map[string]bool{"node12-post-if-always": true},
			postCallOrder: []string{"node12-post-if-always"},
			TestJobFileInfo: TestJobFileInfo{workflowPath: "post/multi-post-when-early-failure", errorMessage: "exit with `FAILURE`: 1"}},
	}

	for _, tjps := range tables {
		workflowPath := tjps.TestJobFileInfo.workflowPath
		t.Run(workflowPath, func(t *testing.T) {
			if testing.Short() {
				t.Skip("skipping integration test")
			}

			log.SetLevel(log.DebugLevel)
			ctx := context.Background()

			platforms := map[string]string{
				"ubuntu-latest": baseImage,
			}

			eventName := "push"

			workdir, err := filepath.Abs("testdata")
			assert.Nil(t, err, workdir)
			fullWorkflowPath := filepath.Join(workdir, workflowPath)
			runnerConfig := &Config{
				Workdir:         workdir,
				BindWorkdir:     false,
				EventName:       eventName,
				Platforms:       platforms,
				ReuseContainers: false,
			}

			providerMock := &actProviderMock{}

			for name, called := range tjps.called {
				if called {
					providerMock.On(fmt.Sprintf("%s_ExecuteNode12PostAction", name), mock.Anything, mock.Anything, mock.Anything).Once()
				}
			}

			providers := &Providers{
				Action: providerMock,
			}
			runner, err := New(runnerConfig, providers)
			assert.Nil(t, err, workflowPath)

			planner, err := model.NewWorkflowPlanner(fullWorkflowPath, true)
			assert.Nil(t, err, fullWorkflowPath)

			plan := planner.PlanEvent(eventName)

			err = runner.NewPlanExecutor(plan)(ctx)
			if tjps.TestJobFileInfo.errorMessage == "" {
				assert.Nil(t, err, fullWorkflowPath)
			} else {
				assert.Error(t, err, tjps.TestJobFileInfo.errorMessage)
			}

			for name, called := range tjps.called {
				if !called {
					providerMock.AssertNotCalled(t, fmt.Sprintf("%s_ExecuteNode12PostAction", name))
				}
			}
			providerMock.AssertExpectations(t)

			// Test Post action call order if present(the inverse of the action order)
			postCalls := make([]mock.Call, 0)
			for _, v := range providerMock.Calls {
				if strings.HasSuffix(v.Method, "_ExecuteNode12PostAction") {
					postCalls = append(postCalls, v)
				}
			}
			for i, name := range tjps.postCallOrder {
				methodName := fmt.Sprintf("%s_ExecuteNode12PostAction", name)
				assert.Equal(t, methodName, postCalls[i].Method, "Post action order failure")
			}
		})
	}
}
