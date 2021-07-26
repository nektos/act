package runner

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/nektos/act/pkg/model"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	assert "github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v3"
)

func TestRunContext_EvalBool(t *testing.T) {
	var yml yaml.Node
	err := yml.Encode(map[string][]interface{}{
		"os":  {"Linux", "Windows"},
		"foo": {"bar", "baz"},
	})
	assert.NoError(t, err)

	hook := test.NewGlobal()
	rc := &RunContext{
		Config: &Config{
			Workdir: ".",
		},
		Env: map[string]string{
			"SOMETHING_TRUE":  "true",
			"SOMETHING_FALSE": "false",
			"SOME_TEXT":       "text",
		},
		Run: &model.Run{
			JobID: "job1",
			Workflow: &model.Workflow{
				Name: "test-workflow",
				Jobs: map[string]*model.Job{
					"job1": {
						Strategy: &model.Strategy{
							RawMatrix: yml,
						},
					},
				},
			},
		},
		Matrix: map[string]interface{}{
			"os":  "Linux",
			"foo": "bar",
		},
		StepResults: map[string]*stepResult{
			"id1": {
				Outputs: map[string]string{
					"foo": "bar",
				},
				Success: true,
			},
		},
	}
	rc.ExprEval = rc.NewExpressionEvaluator()

	tables := []struct {
		in      string
		out     bool
		wantErr bool
	}{
		// The basic ones
		{in: "failure()", out: false},
		{in: "success()", out: true},
		{in: "cancelled()", out: false},
		{in: "always()", out: true},
		{in: "true", out: true},
		{in: "false", out: false},
		{in: "!true", wantErr: true},
		{in: "!false", wantErr: true},
		{in: "1 != 0", out: true},
		{in: "1 != 1", out: false},
		{in: "${{ 1 != 0 }}", out: true},
		{in: "${{ 1 != 1 }}", out: false},
		{in: "1 == 0", out: false},
		{in: "1 == 1", out: true},
		{in: "1 > 2", out: false},
		{in: "1 < 2", out: true},
		// And or
		{in: "true && false", out: false},
		{in: "true && 1 < 2", out: true},
		{in: "false || 1 < 2", out: true},
		{in: "false || false", out: false},
		// None boolable
		{in: "env.UNKNOWN == 'true'", out: false},
		{in: "env.UNKNOWN", out: false},
		// Inline expressions
		{in: "env.SOME_TEXT", out: true}, // this is because Boolean('text') is true in Javascript
		{in: "env.SOME_TEXT == 'text'", out: true},
		{in: "env.SOMETHING_TRUE == 'true'", out: true},
		{in: "env.SOMETHING_FALSE == 'true'", out: false},
		{in: "env.SOMETHING_TRUE", out: true},
		{in: "env.SOMETHING_FALSE", out: true}, // this is because Boolean('text') is true in Javascript
		{in: "!env.SOMETHING_TRUE", wantErr: true},
		{in: "!env.SOMETHING_FALSE", wantErr: true},
		{in: "${{ !env.SOMETHING_TRUE }}", out: false},
		{in: "${{ !env.SOMETHING_FALSE }}", out: false},
		{in: "${{ ! env.SOMETHING_TRUE }}", out: false},
		{in: "${{ ! env.SOMETHING_FALSE }}", out: false},
		{in: "${{ env.SOMETHING_TRUE }}", out: true},
		{in: "${{ env.SOMETHING_FALSE }}", out: true},
		{in: "${{ !env.SOMETHING_TRUE }}", out: false},
		{in: "${{ !env.SOMETHING_FALSE }}", out: false},
		{in: "${{ !env.SOMETHING_TRUE && true }}", out: false},
		{in: "${{ !env.SOMETHING_FALSE && true }}", out: false},
		{in: "${{ !env.SOMETHING_TRUE || true }}", out: true},
		{in: "${{ !env.SOMETHING_FALSE || false }}", out: false},
		{in: "${{ env.SOMETHING_TRUE && true }}", out: true},
		{in: "${{ env.SOMETHING_FALSE || true }}", out: true},
		{in: "${{ env.SOMETHING_FALSE || false }}", out: true},
		{in: "!env.SOMETHING_TRUE || true", wantErr: true},
		{in: "${{ env.SOMETHING_TRUE == 'true'}}", out: true},
		{in: "${{ env.SOMETHING_FALSE == 'true'}}", out: false},
		{in: "${{ env.SOMETHING_FALSE == 'false'}}", out: true},
		{in: "${{ env.SOMETHING_FALSE }} && ${{ env.SOMETHING_TRUE }}", out: true},

		// All together now
		{in: "false || env.SOMETHING_TRUE == 'true'", out: true},
		{in: "true || env.SOMETHING_FALSE == 'true'", out: true},
		{in: "true && env.SOMETHING_TRUE == 'true'", out: true},
		{in: "false && env.SOMETHING_TRUE == 'true'", out: false},
		{in: "env.SOMETHING_FALSE == 'true' && env.SOMETHING_TRUE == 'true'", out: false},
		{in: "env.SOMETHING_FALSE == 'true' && true", out: false},
		{in: "${{ env.SOMETHING_FALSE == 'true' }} && true", out: true},
		{in: "true && ${{ env.SOMETHING_FALSE == 'true' }}", out: true},
		// Check github context
		{in: "github.actor == 'nektos/act'", out: true},
		{in: "github.actor == 'unknown'", out: false},
		// The special ACT flag
		{in: "${{ env.ACT }}", out: true},
		{in: "${{ !env.ACT }}", out: false},
		// Invalid expressions should be reported
		{in: "INVALID_EXPRESSION", wantErr: true},
	}

	updateTestIfWorkflow(t, tables, rc)
	for _, table := range tables {
		table := table
		t.Run(table.in, func(t *testing.T) {
			assertObject := assert.New(t)
			defer hook.Reset()
			b, err := rc.EvalBool(table.in)
			if table.wantErr {
				assertObject.Error(err)
			}

			assertObject.Equal(table.out, b, fmt.Sprintf("Expected %s to be %v, was %v", table.in, table.out, b))
			assertObject.Empty(hook.LastEntry(), table.in)
		})
	}
}

func updateTestIfWorkflow(t *testing.T, tables []struct {
	in      string
	out     bool
	wantErr bool
}, rc *RunContext) {
	var envs string
	keys := make([]string, 0, len(rc.Env))
	for k := range rc.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		envs += fmt.Sprintf("  %s: %s\n", k, rc.Env[k])
	}

	workflow := fmt.Sprintf(`
name: "Test what expressions result in true and false on GitHub"
on: push

env:
%s

jobs:
  test-ifs-and-buts:
    runs-on: ubuntu-latest
    steps:
`, envs)

	for i, table := range tables {
		if table.wantErr || strings.HasPrefix(table.in, "github.actor") {
			continue
		}
		expressionPattern = regexp.MustCompile(`\${{\s*(.+?)\s*}}`)

		expr := expressionPattern.ReplaceAllStringFunc(table.in, func(match string) string {
			return fmt.Sprintf("€{{ %s }}", expressionPattern.ReplaceAllString(match, "$1"))
		})
		echo := fmt.Sprintf(`run: echo "%s should be false, but was evaluated to true;" exit 1;`, table.in)
		name := fmt.Sprintf(`"❌ I should not run, expr: %s"`, expr)
		if table.out {
			echo = `run: echo OK`
			name = fmt.Sprintf(`"✅ I should run, expr: %s"`, expr)
		}
		workflow += fmt.Sprintf("\n      - name: %s\n        id: step%d\n        if: %s\n        %s\n", name, i, table.in, echo)
		if table.out {
			workflow += fmt.Sprintf("\n      - name: \"Double checking expr: %s\"\n        if: steps.step%d.conclusion == 'skipped'\n        run: echo \"%s should have been true, but wasn't\"\n", expr, i, table.in)
		}
	}

	file, err := os.Create("../../.github/workflows/test-if.yml")
	if err != nil {
		t.Fatal(err)
	}

	_, err = file.WriteString(workflow)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunContext_GetBindsAndMounts(t *testing.T) {
	rctemplate := &RunContext{
		Name: "TestRCName",
		Run: &model.Run{
			Workflow: &model.Workflow{
				Name: "TestWorkflowName",
			},
		},
		Config: &Config{
			BindWorkdir: false,
		},
	}

	tests := []struct {
		windowsPath bool
		name        string
		rc          *RunContext
		wantbind    string
		wantmount   string
	}{
		{false, "/mnt/linux", rctemplate, "/mnt/linux", "/mnt/linux"},
		{false, "/mnt/path with spaces/linux", rctemplate, "/mnt/path with spaces/linux", "/mnt/path with spaces/linux"},
		{true, "C:\\Users\\TestPath\\MyTestPath", rctemplate, "/mnt/c/Users/TestPath/MyTestPath", "/mnt/c/Users/TestPath/MyTestPath"},
		{true, "C:\\Users\\Test Path with Spaces\\MyTestPath", rctemplate, "/mnt/c/Users/Test Path with Spaces/MyTestPath", "/mnt/c/Users/Test Path with Spaces/MyTestPath"},
		{true, "/LinuxPathOnWindowsShouldFail", rctemplate, "", ""},
	}

	isWindows := runtime.GOOS == "windows"

	for _, testcase := range tests {
		// pin for scopelint
		testcase := testcase
		for _, bindWorkDir := range []bool{true, false} {
			// pin for scopelint
			bindWorkDir := bindWorkDir
			testBindSuffix := ""
			if bindWorkDir {
				testBindSuffix = "Bind"
			}

			// Only run windows path tests on windows and non-windows on non-windows
			if (isWindows && testcase.windowsPath) || (!isWindows && !testcase.windowsPath) {
				t.Run((testcase.name + testBindSuffix), func(t *testing.T) {
					config := testcase.rc.Config
					config.Workdir = testcase.name
					config.BindWorkdir = bindWorkDir
					gotbind, gotmount := rctemplate.GetBindsAndMounts()

					// Name binds/mounts are either/or
					if config.BindWorkdir {
						fullBind := testcase.name + ":" + testcase.wantbind
						if runtime.GOOS == "darwin" {
							fullBind += ":delegated"
						}
						assert.Contains(t, gotbind, fullBind)
					} else {
						mountkey := testcase.rc.jobContainerName()
						assert.EqualValues(t, testcase.wantmount, gotmount[mountkey])
					}
				})
			}
		}
	}
}

func TestGetGitHubContext(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	cwd, err := os.Getwd()
	assert.Nil(t, err)

	rc := &RunContext{
		Config: &Config{
			EventName: "push",
			Workdir:   cwd,
		},
		Run: &model.Run{
			Workflow: &model.Workflow{
				Name: "GitHubContextTest",
			},
		},
		Name:           "GitHubContextTest",
		CurrentStep:    "step",
		Matrix:         map[string]interface{}{},
		Env:            map[string]string{},
		ExtraPath:      []string{},
		StepResults:    map[string]*stepResult{},
		OutputMappings: map[MappableOutput]MappableOutput{},
	}

	ghc := rc.getGithubContext()

	log.Debugf("%v", ghc)

	actor := "nektos/act"
	if a := os.Getenv("ACT_ACTOR"); a != "" {
		actor = a
	}

	repo := "nektos/act"
	if r := os.Getenv("ACT_REPOSITORY"); r != "" {
		repo = r
	}

	owner := "nektos"
	if o := os.Getenv("ACT_OWNER"); o != "" {
		owner = o
	}

	assert.Equal(t, ghc.RunID, "1")
	assert.Equal(t, ghc.Workspace, rc.Config.containerPath(cwd))
	assert.Equal(t, ghc.RunNumber, "1")
	assert.Equal(t, ghc.RetentionDays, "0")
	assert.Equal(t, ghc.Actor, actor)
	assert.Equal(t, ghc.Repository, repo)
	assert.Equal(t, ghc.RepositoryOwner, owner)
	assert.Equal(t, ghc.RunnerPerflog, "/dev/null")
	assert.Equal(t, ghc.EventPath, ActPath+"/workflow/event.json")
	assert.Equal(t, ghc.Token, rc.Config.Secrets["GITHUB_TOKEN"])
}
