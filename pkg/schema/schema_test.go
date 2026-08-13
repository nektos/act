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

func TestConcurrencyQueue(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "queue single is accepted",
			yaml: `
on: push
concurrency:
  group: g
  queue: single
jobs:
  noop:
    runs-on: ubuntu-latest
    steps:
    - run: echo ok
`,
		},
		{
			name: "queue max is accepted",
			yaml: `
on: push
concurrency:
  group: g
  queue: max
jobs:
  noop:
    runs-on: ubuntu-latest
    steps:
    - run: echo ok
`,
		},
		{
			name: "queue at job level is accepted",
			yaml: `
on: push
jobs:
  noop:
    runs-on: ubuntu-latest
    concurrency:
      group: g
      queue: max
    steps:
    - run: echo ok
`,
		},
		{
			name: "queue with unknown value is rejected",
			yaml: `
on: push
concurrency:
  group: g
  queue: bogus
jobs:
  noop:
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
			if err := yaml.Unmarshal([]byte(tc.yaml), &node); !assert.NoError(t, err) {
				return
			}
			err := (&Node{
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
