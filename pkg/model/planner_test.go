package model

import (
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type TestJobFileInfo struct {
	workflowPath string
	errorMessage string
}

func TestPlanner(t *testing.T) {
	tables := []TestJobFileInfo{
		{"invalid-job-name", "The workflow is not valid. invalid-job-name: Job name invalid-JOB-Name-v1.2.3-docker_hub is invalid. Names must start with a letter or '_' and contain only alphanumeric characters, '-', or '_'"},
		{"empty-workflow", "unable to read workflow, push.yml file is empty: EOF"},

		{"", ""}, // match whole directory
	}
	log.SetLevel(log.DebugLevel)

	workdir, err := filepath.Abs("testdata")
	assert.NoError(t, err, workdir)
	for _, table := range tables {
		fullWorkflowPath := filepath.Join(workdir, table.workflowPath)
		_, err = NewWorkflowPlanner(fullWorkflowPath)
		if table.errorMessage == "" {
			assert.NoError(t, err, "WorkflowPlanner should exit without any error")
		} else {
			assert.EqualError(t, err, table.errorMessage)
		}
	}
}
