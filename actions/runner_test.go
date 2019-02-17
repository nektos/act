package actions

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	"gotest.tools/assert"
)

func TestRunEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tables := []struct {
		workflowPath string
		eventName    string
		errorMessage string
	}{
		{"basic.workflow", "push", ""},
		{"pipe.workflow", "push", ""},
		{"fail.workflow", "push", "exit with `FAILURE`: 1"},
		{"regex.workflow", "push", "exit with `NEUTRAL`: 78"},
		{"gitref.workflow", "push", ""},
		{"env.workflow", "push", ""},
		{"detect_event.workflow", "", ""},
	}
	log.SetLevel(log.DebugLevel)

	for _, table := range tables {
		runnerConfig := &RunnerConfig{
			Ctx:          context.Background(),
			WorkflowPath: table.workflowPath,
			WorkingDir:   "testdata",
			EventName:    table.eventName,
		}
		runner, err := NewRunner(runnerConfig)
		assert.NilError(t, err, table.workflowPath)

		err = runner.RunEvent()
		if table.errorMessage == "" {
			assert.NilError(t, err, table.workflowPath)
		} else {
			assert.Error(t, err, table.errorMessage)
		}
	}
}
