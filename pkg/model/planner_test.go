package model

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type WorkflowPlanTest struct {
	workflowPath      string
	errorMessage      string
	warnMessage       string
	noWorkflowRecurse bool
}

var getLog GetLog

type GetLog struct {
	buffer          bytes.Buffer
	oldLoggerOutput io.Writer
	isLock          bool
}

func (g *GetLog) Lock() {
	if g.isLock {
		log.Fatal("log is already locked")
		return
	}
	g.isLock = true
	g.oldLoggerOutput = log.StandardLogger().Out
	log.SetOutput(&g.buffer)
}

func (g *GetLog) Unlock() string {
	if !g.isLock {
		log.Fatal("log is not locked")
		return ""
	}
	g.isLock = false
	log.SetOutput(g.oldLoggerOutput)
	return g.buffer.String()
}

func TestPlanner(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	tables := []WorkflowPlanTest{
		{"invalid-job-name/invalid-1.yml", "workflow is not valid. 'invalid-job-name-1': Job name 'invalid-JOB-Name-v1.2.3-docker_hub' is invalid. Names must start with a letter or '_' and contain only alphanumeric characters, '-', or '_'", "-", false},
		{"invalid-job-name/invalid-2.yml", "workflow is not valid. 'invalid-job-name-2': Job name '1234invalid-JOB-Name-v123-docker_hub' is invalid. Names must start with a letter or '_' and contain only alphanumeric characters, '-', or '_'", "-", false},
		{"invalid-job-name/valid-1.yml", "", "-", false},
		{"invalid-job-name/valid-2.yml", "", "-", false},
		{"empty-workflow", "", "unable to read workflow 'push.yml': file is empty: EOF", false},
		{"nested", "", "unable to read workflow 'push.yml': file is empty: EOF", false},
		{"nested", "", "-", true},
	}

	workdir, err := filepath.Abs("testdata")
	assert.NoError(t, err, workdir)
	for _, table := range tables {
		fullWorkflowPath := filepath.Join(workdir, table.workflowPath)
		getLog.Lock()
		_, err = NewWorkflowPlanner(fullWorkflowPath, table.noWorkflowRecurse)
		warnMessage := getLog.Unlock()

		// Check if the expected warning message is present in the log output
		if table.warnMessage != "-" {
			assert.True(t, strings.Contains(warnMessage, table.warnMessage))
		}

		// Check if an error is expected and if so, assert that the error matches the expected message
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
