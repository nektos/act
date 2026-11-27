package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestAdditionalFunctions(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    if: success() || success('joba', 'jobb') || failure() || failure('joba', 'jobb') || always() || cancelled()
    steps:
    - run: exit 0
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestAdditionalFunctionsFailure(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    if: success() || success('joba', 'jobb') || failure() || failure('joba', 'jobb') || always('error')
    steps:
    - run: exit 0
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.Error(t, err)
}

func TestAdditionalFunctionsSteps(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    steps:
    - run: exit 0
      if: success() || failure() || always()
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestAdditionalFunctionsStepsExprSyntax(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
on: push
jobs:
  job-with-condition:
    runs-on: self-hosted
    steps:
    - run: exit 0
      if: ${{ success() || failure() || always() }}
`), &node)
	if !assert.NoError(t, err) {
		return
	}
	err = (&Node{
		Definition: "workflow-root-strict",
		Schema:     GetWorkflowSchema(),
	}).UnmarshalYAML(&node)
	assert.NoError(t, err)
}

func TestCacheModeWorkflowAndJob(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "workflow-level none",
			yaml: `
on: push
cache-mode: none
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - run: echo ok
`,
		},
		{
			name: "workflow-level write-only",
			yaml: `
on: push
cache-mode: write-only
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - run: echo ok
`,
		},
		{
			name: "job-level read",
			yaml: `
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    cache-mode: read
    steps:
    - run: echo ok
`,
		},
		{
			name: "reusable workflow job cache-mode",
			yaml: `
on: push
jobs:
  call:
    uses: org/repo/.github/workflows/reuse.yml@v1
    cache-mode: write
`,
		},
		{
			name: "invalid workflow cache-mode",
			yaml: `
on: push
cache-mode: disabled
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - run: echo ok
`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(tc.yaml), &node)
			if !assert.NoError(t, err) {
				return
			}
			err = (&Node{
				Definition: "workflow-root-strict",
				Schema:     GetWorkflowSchema(),
			}).UnmarshalYAML(&node)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
