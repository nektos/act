package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadWorkflow_StringEvent(t *testing.T) {
	yaml := `
name: local-action-docker-url
on: push

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")

	assert.Len(t, workflow.On(), 1)
	assert.Contains(t, workflow.On(), "push")
}

func TestReadWorkflow_ListEvent(t *testing.T) {
	yaml := `
name: local-action-docker-url
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")

	assert.Len(t, workflow.On(), 2)
	assert.Contains(t, workflow.On(), "push")
	assert.Contains(t, workflow.On(), "pull_request")
}

func TestReadWorkflow_MapEvent(t *testing.T) {
	yaml := `
name: local-action-docker-url
on:
  push:
    branches:
    - master
  pull_request:
    branches:
    - master

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.On(), 2)
	assert.Contains(t, workflow.On(), "push")
	assert.Contains(t, workflow.On(), "pull_request")
}

func TestGetWorkflowFilterStrings(t *testing.T) {
	testCases := []struct {
		name           string
		yaml           string
		inputEvent     string
		expectedOutput *FilterPatterns
	}{
		{
			name: "on.push.branches",
			yaml: `
name: local-action-docker-url
on:
  push:
    branches:
    - master

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{Branches: []string{"master"}},
		},
		{
			name: "on.push.branches - alternate syntax",
			yaml: `
name: local-action-docker-url
on:
  push:
    branches: [master]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{Branches: []string{"master"}},
		},
		{
			name: "on.push.branches-ignore",
			yaml: `
name: local-action-docker-url
on:
  push:
    branches-ignore:
    - "**test"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{BranchesIgnore: []string{"**test"}},
		},
		{
			name: "on.push.tags",
			yaml: `
name: local-action-docker-url
on:
  push:
    tags:
    - "*-release"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{Tags: []string{"*-release"}},
		},
		{
			name: "on.push.tags - alternate syntax",
			yaml: `
name: local-action-docker-url
on:
  push:
    tags: ["*-release"]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{Tags: []string{"*-release"}},
		},
		{
			name: "on.push.tags-ignore",
			yaml: `
name: local-action-docker-url
on:
  push:
    tags-ignore:
    - "*-alpha"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{TagsIgnore: []string{"*-alpha"}},
		},
		{
			name: "on.push.paths",
			yaml: `
name: local-action-docker-url
on:
  push:
    paths:
    - "**.go"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{Paths: []string{"**.go"}},
		},
		{
			name: "on.push.paths - alternate syntax",
			yaml: `
name: local-action-docker-url
on:
  push:
    paths: ["**.go"]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{Paths: []string{"**.go"}},
		},
		{
			name: "on.push.paths-ignore",
			yaml: `
name: local-action-docker-url
on:
  push:
    paths-ignore:
    - "**.md"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{PathsIgnore: []string{"**.md"}},
		},
		{
			name: "on.pull_request.branches",
			yaml: `
name: local-action-docker-url
on:
  pull_request:
    branches:
    - master

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "pull_request",
			expectedOutput: &FilterPatterns{Branches: []string{"master"}},
		},
		{
			name: "on.pull_request.branches - alternate syntax",
			yaml: `
name: local-action-docker-url
on:
  pull_request:
    branches: [master]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "pull_request",
			expectedOutput: &FilterPatterns{Branches: []string{"master"}},
		},
		{
			name: "on.pull_request.paths",
			yaml: `
name: local-action-docker-url
on:
  pull_request:
    paths:
    - "**.go"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "pull_request",
			expectedOutput: &FilterPatterns{Paths: []string{"**.go"}},
		},
		{
			name: "on.pull_request.paths",
			yaml: `
name: local-action-docker-url
on:
  pull_request:
    paths: ["**.go"]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "pull_request",
			expectedOutput: &FilterPatterns{Paths: []string{"**.go"}},
		},
		{
			name: "on.push.tags AND on.push.paths",
			yaml: `
name: local-action-docker-url
on:
  push:
    tags:
    - "*-release"
    paths:
    - "**.go"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{Paths: []string{"**.go"}, Tags: []string{"*-release"}},
		},
		{
			name: "on.push.tags AND on.push.paths - alternate syntax",
			yaml: `
name: local-action-docker-url
on:
  push:
    tags: ["*-release"]
    paths: ["**.go"]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: &FilterPatterns{Paths: []string{"**.go"}, Tags: []string{"*-release"}},
		},
		{
			name: "on.pull_request.branches AND on.pull_request.paths",
			yaml: `
name: local-action-docker-url
on:
  pull_request:
    branches:
    - master
    paths:
    - "**.go"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "pull_request",
			expectedOutput: &FilterPatterns{Branches: []string{"master"}, Paths: []string{"**.go"}},
		},
		{
			name: "on.pull_request.branches AND on.pull_request.paths - alternate syntax",
			yaml: `
name: local-action-docker-url
on:
  pull_request:
    branches: [master]
    paths: ["**.go"]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "pull_request",
			expectedOutput: &FilterPatterns{Branches: []string{"master"}, Paths: []string{"**.go"}},
		},
		{
			name: "on.pull_request.branches AND on.push.tags",
			yaml: `
name: local-action-docker-url
on:
  pull_request:
    branches:
    - "master"
    - "rc"
  push:
    tags:
    - "*-release"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "pull_request",
			expectedOutput: &FilterPatterns{Branches: []string{"master", "rc"}},
		},
		{
			name: "on.pull_request.branches AND on.push.tags - alternate syntax",
			yaml: `
name: local-action-docker-url
on:
  pull_request:
    branches: ["master", "rc"]
  push:
    tags: ["*-release"]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "pull_request",
			expectedOutput: &FilterPatterns{Branches: []string{"master", "rc"}},
		},
		{
			name: "on.push - no filters supplied",
			yaml: `
name: local-action-docker-url
on: push

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "push",
			expectedOutput: nil,
		},
		{
			name: "on.schedule - filters not supported",
			yaml: `
name: local-action-docker-url
on:
  schedule:
  - cron: $cron-weekly

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: ./actions/docker-url
`,
			inputEvent:     "schedule",
			expectedOutput: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workflow, err := ReadWorkflow(strings.NewReader(tc.yaml))
			assert.NoError(t, err, "read workflow should succeed")

			assert.Equal(t, tc.expectedOutput, workflow.FindFilterPatterns(tc.inputEvent))
		})
	}
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

func TestReadWorkflow_JobTypes(t *testing.T) {
	yaml := `
name: invalid job definition

jobs:
  default-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo
  remote-reusable-workflow:
    runs-on: ubuntu-latest
    uses: remote/repo/.github/workflows/workflow.yml@main
  local-reusable-workflow:
    runs-on: ubuntu-latest
    uses: ./.github/workflows/workflow.yml
`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.Jobs, 3)
	assert.Equal(t, workflow.Jobs["default-job"].Type(), JobTypeDefault)
	assert.Equal(t, workflow.Jobs["remote-reusable-workflow"].Type(), JobTypeReusableWorkflowRemote)
	assert.Equal(t, workflow.Jobs["local-reusable-workflow"].Type(), JobTypeReusableWorkflowLocal)
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
	yaml := `
name: job outputs definition

jobs:
  test1:
    runs-on: ubuntu-latest
    steps:
      - id: test1_1
        run: |
          echo "::set-output name=a_key::some-a_value"
          echo "::set-output name=b-key::some-b-value"
    outputs:
      some_a_key: ${{ steps.test1_1.outputs.a_key }}
      some-b-key: ${{ steps.test1_1.outputs.b-key }}

  test2:
    runs-on: ubuntu-latest
    needs:
      - test1
    steps:
      - name: test2_1
        run: |
          echo "${{ needs.test1.outputs.some_a_key }}"
          echo "${{ needs.test1.outputs.some-b-key }}"
`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.Jobs, 2)

	assert.Len(t, workflow.Jobs["test1"].Steps, 1)
	assert.Equal(t, StepTypeRun, workflow.Jobs["test1"].Steps[0].Type())
	assert.Equal(t, "test1_1", workflow.Jobs["test1"].Steps[0].ID)
	assert.Len(t, workflow.Jobs["test1"].Outputs, 2)
	assert.Contains(t, workflow.Jobs["test1"].Outputs, "some_a_key")
	assert.Contains(t, workflow.Jobs["test1"].Outputs, "some-b-key")
	assert.Equal(t, "${{ steps.test1_1.outputs.a_key }}", workflow.Jobs["test1"].Outputs["some_a_key"])
	assert.Equal(t, "${{ steps.test1_1.outputs.b-key }}", workflow.Jobs["test1"].Outputs["some-b-key"])
}

func TestReadWorkflow_Strategy(t *testing.T) {
	w, err := NewWorkflowPlanner("testdata/strategy/push.yml", true)
	assert.NoError(t, err)

	p, err := w.PlanJob("strategy-only-max-parallel")
	assert.NoError(t, err)

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
			{"php-version": 5.4},
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
