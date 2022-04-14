package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	workdir = "../testdata"
)

func init() {
	if wd, err := filepath.Abs(workdir); err == nil {
		workdir = wd
	}
}

func readWorkflow(t *testing.T, name string) (w *Workflow) {
	f, err := os.OpenFile(filepath.Join(workdir, name), os.O_RDONLY, 0)
	if err != nil {
		assert.NoError(t, err, "file open should succeed")
	}

	w, err = ReadWorkflow(f)
	if err != nil {
		assert.NoError(t, err, "read workflow should succeed")
	}

	return w
}

func TestReadWorkflow_Event(t *testing.T) {
	t.Run("on-string", func(t *testing.T) {
		w := readWorkflow(t, "event/string.yml")

		assert.Len(t, w.On(), 1)
		assert.Contains(t, w.On(), "push")
	})

	t.Run("on-list", func(t *testing.T) {
		w := readWorkflow(t, "event/list.yml")

		assert.Len(t, w.On(), 3)
		assert.Contains(t, w.On(), "push")
		assert.Contains(t, w.On(), "pull_request")
		assert.Contains(t, w.On(), "workflow_dispatch")
	})

	t.Run("on-map", func(t *testing.T) {
		w := readWorkflow(t, "event/map.yml")

		assert.Len(t, w.On(), 2)
		assert.Contains(t, w.On(), "push")
		assert.Contains(t, w.On(), "pull_request")
	})
}

func TestReadWorkflow_StringContainer(t *testing.T) {
	yaml := `
name: local-action-docker-url

jobs:
  test:
    container: nginx:latest
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
  test2:
    container:
      image: nginx:latest
      env:
        foo: bar
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.Jobs, 2)
	assert.Contains(t, workflow.Jobs["test"].Container().Image, "nginx:latest")
	assert.Contains(t, workflow.Jobs["test2"].Container().Image, "nginx:latest")
	assert.Contains(t, workflow.Jobs["test2"].Container().Env["foo"], "bar")
}

func TestReadWorkflow_ObjectContainer(t *testing.T) {
	yaml := `
name: local-action-docker-url

jobs:
  test:
    container:
      image: r.example.org/something:latest
      credentials:
        username: registry-username
        password: registry-password
      env:
        HOME: /home/user
      volumes:
        - my_docker_volume:/volume_mount
        - /data/my_data
        - /source/directory:/destination/directory
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.Jobs, 1)

	container := workflow.GetJob("test").Container()

	assert.Contains(t, container.Image, "r.example.org/something:latest")
	assert.Contains(t, container.Env["HOME"], "/home/user")
	assert.Contains(t, container.Credentials["username"], "registry-username")
	assert.Contains(t, container.Credentials["password"], "registry-password")
	assert.ElementsMatch(t, container.Volumes, []string{
		"my_docker_volume:/volume_mount",
		"/data/my_data",
		"/source/directory:/destination/directory",
	})
}

func TestReadWorkflow_StepsTypes(t *testing.T) {
	yaml := `
name: invalid step definition

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: test1
        uses: actions/checkout@v2
        run: echo
      - name: test2
        run: echo
      - name: test3
        uses: actions/checkout@v2
      - name: test4
        uses: docker://nginx:latest
      - name: test5
        uses: ./local-action
`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.Jobs, 1)
	assert.Len(t, workflow.Jobs["test"].Steps, 5)
	assert.Equal(t, workflow.Jobs["test"].Steps[0].Type(), StepTypeInvalid)
	assert.Equal(t, workflow.Jobs["test"].Steps[1].Type(), StepTypeRun)
	assert.Equal(t, workflow.Jobs["test"].Steps[2].Type(), StepTypeUsesActionRemote)
	assert.Equal(t, workflow.Jobs["test"].Steps[3].Type(), StepTypeUsesDockerURL)
	assert.Equal(t, workflow.Jobs["test"].Steps[4].Type(), StepTypeUsesActionLocal)
}

// See: https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions#jobsjob_idoutputs
func TestReadWorkflow_JobOutputs(t *testing.T) {
	w := readWorkflow(t, "outputs/push.yml")

	assert.Len(t, w.Jobs, 2)

	j := w.Jobs["build_output"]
	assert.Len(t, j.Steps, 3)
	assert.Equal(t, StepTypeRun, j.Steps[0].Type())
	assert.Equal(t, "set_1", j.Steps[0].ID)
	assert.Equal(t, "set_2", j.Steps[1].ID)
	assert.Equal(t, "set_3", j.Steps[2].ID)

	assert.Len(t, j.Outputs, 4)
	assert.Equal(t, map[string]string{
		"variable_1": "${{ steps.set_1.outputs.var_1 }}",
		"variable_2": "${{ steps.set_1.outputs.var_2 }}",
		"variable_3": "${{ steps.set_2.outputs.var_3 }}",
		"variable_4": "${{ steps.set_3.outputs.var_4 }}",
	}, j.Outputs)
}

func TestReadWorkflow_Strategy(t *testing.T) {
	w, err := NewWorkflowPlanner("../testdata/strategy/push.yml", true)
	assert.NoError(t, err)

	p := w.PlanJob("strategy-only-max-parallel")

	assert.Equal(t, len(p.Stages), 1)
	assert.Equal(t, len(p.Stages[0].Runs), 1)

	wf := p.Stages[0].Runs[0].Workflow

	job := wf.Jobs["strategy-only-max-parallel"]
	assert.Equal(t, job.GetMatrixes(), []map[string]interface{}{{}})
	assert.Equal(t, job.Matrix(), map[string][]interface{}(nil))
	assert.Equal(t, job.Strategy.MaxParallel, 2)
	assert.Equal(t, job.Strategy.FailFast, true)

	job = wf.Jobs["strategy-only-fail-fast"]
	assert.Equal(t, job.GetMatrixes(), []map[string]interface{}{{}})
	assert.Equal(t, job.Matrix(), map[string][]interface{}(nil))
	assert.Equal(t, job.Strategy.MaxParallel, 4)
	assert.Equal(t, job.Strategy.FailFast, false)

	job = wf.Jobs["strategy-no-matrix"]
	assert.Equal(t, job.GetMatrixes(), []map[string]interface{}{{}})
	assert.Equal(t, job.Matrix(), map[string][]interface{}(nil))
	assert.Equal(t, job.Strategy.MaxParallel, 2)
	assert.Equal(t, job.Strategy.FailFast, false)

	job = wf.Jobs["strategy-all"]
	assert.Equal(t, job.GetMatrixes(),
		[]map[string]interface{}{
			{"datacenter": "site-c", "node-version": "14.x", "site": "staging"},
			{"datacenter": "site-c", "node-version": "16.x", "site": "staging"},
			{"datacenter": "site-d", "node-version": "16.x", "site": "staging"},
			{"datacenter": "site-a", "node-version": "10.x", "site": "prod"},
			{"datacenter": "site-b", "node-version": "12.x", "site": "dev"},
		},
	)
	assert.Equal(t, job.Matrix(),
		map[string][]interface{}{
			"datacenter": {"site-c", "site-d"},
			"exclude": {
				map[string]interface{}{"datacenter": "site-d", "node-version": "14.x", "site": "staging"},
			},
			"include": {
				map[string]interface{}{"php-version": 5.4},
				map[string]interface{}{"datacenter": "site-a", "node-version": "10.x", "site": "prod"},
				map[string]interface{}{"datacenter": "site-b", "node-version": "12.x", "site": "dev"},
			},
			"node-version": {"14.x", "16.x"},
			"site":         {"staging"},
		},
	)
	assert.Equal(t, job.Strategy.MaxParallel, 2)
	assert.Equal(t, job.Strategy.FailFast, false)
}

func TestStep_ShellCommand(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"pwsh -v '. {0}'", "pwsh -v '. {0}'"},
		{"pwsh", "pwsh -command . '{0}'"},
		{"powershell", "powershell -command . '{0}'"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			got := (&Step{Shell: tt.shell}).ShellCommand()
			assert.Equal(t, got, tt.want)
		})
	}
}
