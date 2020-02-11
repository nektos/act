package model

import (
	"strings"
	"testing"

	"gotest.tools/assert"
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
	assert.NilError(t, err, "read workflow should succeed")

	assert.DeepEqual(t, workflow.On(), []string{"push"})
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
	assert.NilError(t, err, "read workflow should succeed")

	assert.DeepEqual(t, workflow.On(), []string{"push", "pull_request"})
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
	assert.NilError(t, err, "read workflow should succeed")

	assert.DeepEqual(t, workflow.On(), []string{"push", "pull_request"})
}
