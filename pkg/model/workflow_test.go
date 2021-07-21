package model

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadWorkflow_StringEvent(t *testing.T) {
	yaml := `on: push`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")

	assert.Len(t, workflow.On(), 1)
	assert.Contains(t, workflow.On(), "push")
}

func TestReadWorkflow_ListEvent(t *testing.T) {
	yaml := `on: [push, pull_request]`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")

	assert.Len(t, workflow.On(), 2)
	assert.Contains(t, workflow.On(), "push")
	assert.Contains(t, workflow.On(), "pull_request")
}

func TestReadWorkflow_MapEvent(t *testing.T) {
	yaml := `
on:
  push:
    branches:
    - master
  pull_request:
    branches:
    - master
`

	workflow, err := ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.On(), 2)
	assert.Contains(t, workflow.On(), "push")
	assert.Contains(t, workflow.On(), "pull_request")
}

func TestReadWorkflow_StringContainer(t *testing.T) {
	f, err := os.Open("testdata/complete-workflow/main.yml")
	assert.Nil(t, err)

	workflow, err := ReadWorkflow(f)
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.Jobs, 6)
	assert.Contains(t, workflow.Jobs["container-job"].Container().Image, "nginx:latest")
	assert.Contains(t, workflow.Jobs["container-execution-context"].Container().Image, "nginx:latest")
	assert.Contains(t, workflow.Jobs["container-execution-context"].Container().Env["foo"], "bar")
}

func TestReadWorkflow_StepsTypes(t *testing.T) {
	f, err := os.Open("testdata/complete-workflow/main.yml")
	assert.Nil(t, err)

	workflow, err := ReadWorkflow(f)
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.Jobs, 6)
	stepTest := workflow.Jobs["invalid-step-definition"]
	assert.Len(t, stepTest.Steps, 5)
	assert.Equal(t, stepTest.Steps[0].Type(), StepTypeInvalid)
	assert.Equal(t, stepTest.Steps[1].Type(), StepTypeRun)
	assert.Equal(t, stepTest.Steps[2].Type(), StepTypeUsesActionRemote)
	assert.Equal(t, stepTest.Steps[3].Type(), StepTypeUsesDockerURL)
	assert.Equal(t, stepTest.Steps[4].Type(), StepTypeUsesActionLocal)
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

func TestStep_ShellCommand(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"bash", "bash --noprofile --norc -e -o pipefail {0}"},
		{"", "bash --noprofile --norc -e -o pipefail {0}"},
		{"pwsh", "pwsh -command . '{0}'"},
		{"python", "python {0}"},
		{"sh", "sh -e -c {0}"},
		{"cmd", "%ComSpec% /D /E:ON /V:OFF /S /C \"CALL \"{0}\"\""},
		{"powershell", "powershell -command . '{0}'"},
		{"custom", "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			got := (&Step{Shell: tt.shell}).ShellCommand()
			assert.Equal(t, got, tt.want)
		})
	}
}

type TestEnvTable struct {
	give interface{}
	want map[string]string
}

func TestEnvironment(t *testing.T) {
	for _, v := range []TestEnvTable{
		{
			give: map[string]interface{}{
				"var1": "stringValue",
				"var2": map[string]string{
					"var1": "stringInInterface",
				},
				"var3": 5,
				"var4": true,
			},
			want: map[string]string{
				"var1": "stringValue",
				"var3": "5",
				"var4": "true",
			},
		},
		{
			give: map[string]string{
				"var1": "stringValue",
				"var2": "secondString",
			},
			want: map[string]string{
				"var1": "stringValue",
				"var2": "secondString",
			},
		},
		{
			give: "var1",
			want: map[string]string{},
		},
		{
			give: []string{
				"var1", "var2", "var3",
			},
			want: map[string]string{},
		},
		{
			give: 5,
			want: map[string]string{},
		},
		{
			give: []byte("byte"),
			want: map[string]string{},
		},
	} {
		j := Job{Env: v.give}
		s := Step{Env: v.give}

		assert.Equal(t, j.Environment(), v.want)
		assert.Equal(t, s.Environment(), v.want)
		assert.Equal(t, environment(v.give), v.want)
	}

	f, err := os.Open("testdata/complete-workflow/main.yml")
	assert.Nil(t, err)

	workflow, err := ReadWorkflow(f)
	assert.NoError(t, err, "read workflow should succeed")
	assert.Len(t, workflow.Jobs, 6)
	stepTest := workflow.Jobs["env-test-job"]
	assert.Equal(t, environment(stepTest.Env), map[string]string{"BOOLEAN": "true", "INT": "5", "STRING": "string"})
	assert.Len(t, stepTest.Steps, 2)
	assert.Equal(t, environment(stepTest.Steps[0].Env), map[string]string{"BOOLEAN": "true", "INT": "5", "STRING": "string"})
	assert.Equal(t, stepTest.Steps[1].GetEnv(), map[string]string{"INPUT_BOOLEAN": "true", "INPUT_INT": "5", "INPUT_STRING": "string"})
}
