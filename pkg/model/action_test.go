package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// TestActionRunsUsing_AcceptedValues ensures every supported runs.using value
// (composite, docker, node12, node16, node20, node24) is accepted by the
// YAML unmarshaller. node24 was added under CI-21548.
func TestActionRunsUsing_AcceptedValues(t *testing.T) {
	cases := []string{
		ActionRunsUsingComposite,
		ActionRunsUsingDocker,
		ActionRunsUsingNode12,
		ActionRunsUsingNode16,
		ActionRunsUsingNode20,
		ActionRunsUsingNode24,
	}

	for _, using := range cases {
		t.Run(using, func(t *testing.T) {
			yamlBody := "using: " + using + "\n"
			var runs ActionRuns
			err := yaml.Unmarshal([]byte(yamlBody), &runs)
			assert.NoError(t, err, "runs.using: %s should be accepted", using)
			assert.Equal(t, ActionRunsUsing(using), runs.Using)
		})
	}
}

// TestActionRunsUsing_CaseInsensitive checks that node24 is accepted regardless
// of case, matching the behaviour for node20 and the other node versions.
func TestActionRunsUsing_CaseInsensitive(t *testing.T) {
	cases := []string{"Node24", "NODE24", "nOdE24"}
	for _, using := range cases {
		t.Run(using, func(t *testing.T) {
			yamlBody := "using: " + using + "\n"
			var runs ActionRuns
			err := yaml.Unmarshal([]byte(yamlBody), &runs)
			assert.NoError(t, err, "runs.using: %s should be accepted", using)
			assert.Equal(t, ActionRunsUsing(ActionRunsUsingNode24), runs.Using)
		})
	}
}

// TestActionRunsUsing_RejectsUnsupported makes sure the allow-list still
// rejects garbage values and that the error message enumerates the supported
// values including node24.
func TestActionRunsUsing_RejectsUnsupported(t *testing.T) {
	yamlBody := "using: node99\n"
	var runs ActionRuns
	err := yaml.Unmarshal([]byte(yamlBody), &runs)

	assert.Error(t, err, "runs.using: node99 should be rejected")
	assert.Contains(t, err.Error(), "node24",
		"error message should advertise node24 as a supported value")
	assert.Contains(t, err.Error(), "got node99")
}

// TestReadAction_Node24 checks the full ReadAction path with a realistic
// node24 action.yml body.
func TestReadAction_Node24(t *testing.T) {
	yamlBody := `
name: some-node24-action
description: fake action for testing
runs:
  using: node24
  main: index.js
`
	action, err := ReadAction(strings.NewReader(yamlBody))
	assert.NoError(t, err, "ReadAction should accept runs.using: node24")
	assert.Equal(t, ActionRunsUsing(ActionRunsUsingNode24), action.Runs.Using)
	assert.Equal(t, "index.js", action.Runs.Main)
}
