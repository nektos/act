package model

import (
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type WorkflowPlanTest struct {
	workflowPath      string
	errorMessage      string
	noWorkflowRecurse bool
}

func TestPlanner(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	tables := []WorkflowPlanTest{
		{"invalid-job-name/invalid-1.yml", "workflow is not valid. 'invalid-job-name-1': Job name 'invalid-JOB-Name-v1.2.3-docker_hub' is invalid. Names must start with a letter or '_' and contain only alphanumeric characters, '-', or '_'", false},
		{"invalid-job-name/invalid-2.yml", "workflow is not valid. 'invalid-job-name-2': Job name '1234invalid-JOB-Name-v123-docker_hub' is invalid. Names must start with a letter or '_' and contain only alphanumeric characters, '-', or '_'", false},
		{"invalid-job-name/valid-1.yml", "", false},
		{"invalid-job-name/valid-2.yml", "", false},
		{"empty-workflow", "unable to read workflow 'push.yml': file is empty: EOF", false},
		{"nested", "unable to read workflow 'fail.yml': file is empty: EOF", false},
		{"nested", "", true},
	}

	workdir, err := filepath.Abs("testdata")
	assert.NoError(t, err, workdir)
	for _, table := range tables {
		fullWorkflowPath := filepath.Join(workdir, table.workflowPath)
		_, err = NewWorkflowPlanner(fullWorkflowPath, table.noWorkflowRecurse)
		if table.errorMessage == "" {
			assert.NoError(t, err, "WorkflowPlanner should exit without any error")
		} else {
			assert.EqualError(t, err, table.errorMessage)
		}
	}
}

func TestWorkflow(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	workflow := Workflow{
		Jobs: map[string]*Job{
			"valid_job": {
				Name: "valid_job",
			},
		},
	}

	// Check that an invalid job id returns error
	result, err := createStages(&workflow, "invalid_job_id")
	assert.NotNil(t, err)
	assert.Nil(t, result)

	// Check that an valid job id returns non-error
	result, err = createStages(&workflow, "valid_job")
	assert.Nil(t, err)
	assert.NotNil(t, result)
}
