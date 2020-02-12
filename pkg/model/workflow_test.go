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
