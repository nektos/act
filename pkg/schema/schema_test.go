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

func TestJobEnvironmentDeployment(t *testing.T) {
	tests := []struct {
		name       string
		deployment string
		wantErr    string
	}{
		{name: "false", deployment: "false"},
		{name: "true", deployment: "true"},
		{name: "not a boolean", deployment: "maybe", wantErr: "into bool"},
	}
	for _, definition := range []string{"workflow-root", "workflow-root-strict"} {
		for _, tt := range tests {
			t.Run(definition+"/"+tt.name, func(t *testing.T) {
				var node yaml.Node
				err := yaml.Unmarshal([]byte(`
on: push
jobs:
  deploy:
    runs-on: ubuntu-latest
    environment:
      name: production
      deployment: `+tt.deployment+`
    steps:
    - run: exit 0
`), &node)
				if !assert.NoError(t, err) {
					return
				}
				err = (&Node{
					Definition: definition,
					Schema:     GetWorkflowSchema(),
				}).UnmarshalYAML(&node)
				if tt.wantErr == "" {
					assert.NoError(t, err)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}
			})
		}
	}
}
