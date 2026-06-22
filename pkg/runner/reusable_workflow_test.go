package runner

import (
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRemoteReusableWorkflow_SingleRemoteWorkflow(t *testing.T) {
	rc := createReusableWorkflowRunContext("test", "owner/repo/.github/workflows/workflow.yml@v1", nil)

	result := newRemoteReusableWorkflow(rc)

	assertRemoteWorkflow(t, result, "owner", "repo", "workflow.yml", "v1")
}

func TestNewRemoteReusableWorkflow_NestedWithRelativePath(t *testing.T) {
	callerRC := createReusableWorkflowRunContext("caller", "owner/repo/.github/workflows/parent.yml@v1", nil)
	nestedRC := createReusableWorkflowRunContext("nested", "./.github/workflows/nested.yml", callerRC)

	result := newRemoteReusableWorkflow(nestedRC)

	assertRemoteWorkflow(t, result, "owner", "repo", "nested.yml", "v1")
}

func TestNewRemoteReusableWorkflow_NestedWithDifferentRemotePath(t *testing.T) {
	callerRC := createReusableWorkflowRunContext("caller", "owner1/repo1/.github/workflows/parent.yml@v1", nil)
	nestedRC := createReusableWorkflowRunContext("nested", "owner2/repo2/.github/workflows/nested.yml@v2", callerRC)

	result := newRemoteReusableWorkflow(nestedRC)

	assertRemoteWorkflow(t, result, "owner2", "repo2", "nested.yml", "v2")
}

func TestNewRemoteReusableWorkflow_NestedWithDifferentRemotePathAndNestedRelative(t *testing.T) {
	// Three-level nesting: grandparent -> parent (different remote) -> child (relative)
	grandparentRC := createReusableWorkflowRunContext("grandparent", "owner1/repo1/.github/workflows/grandparent.yml@v1", nil)
	parentRC := createReusableWorkflowRunContext("parent", "owner2/repo2/.github/workflows/parent.yml@v2", grandparentRC)
	childRC := createReusableWorkflowRunContext("child", "./.github/workflows/child.yml", parentRC)

	result := newRemoteReusableWorkflow(childRC)

	assertRemoteWorkflow(t, result, "owner2", "repo2", "child.yml", "v2")
}

func TestNewRemoteReusableWorkflow_TripleNestedWithRelativePaths(t *testing.T) {
	// Three-level nesting: grandparent -> parent (relative) -> child (relative)
	grandparentRC := createReusableWorkflowRunContext("grandparent", "owner1/repo1/.github/workflows/grandparent.yml@v1", nil)
	parentRC := createReusableWorkflowRunContext("parent", "./.github/workflows/parent.yml", grandparentRC)
	childRC := createReusableWorkflowRunContext("child", "./.github/workflows/child.yml", parentRC)

	result := newRemoteReusableWorkflow(childRC)

	assertRemoteWorkflow(t, result, "owner1", "repo1", "child.yml", "v1")
}

func createReusableWorkflowRunContext(jobID, uses string, callerRC *RunContext) *RunContext {
	job := &model.Job{
		Uses: uses,
	}

	workflow := &model.Workflow{
		Jobs: map[string]*model.Job{
			jobID: job,
		},
	}

	run := &model.Run{
		Workflow: workflow,
		JobID:    jobID,
	}

	rc := &RunContext{
		Run:    run,
		Config: &Config{},
	}

	if callerRC != nil {
		rc.caller = &caller{
			runContext: callerRC,
		}
	}

	return rc
}

func assertRemoteWorkflow(t *testing.T, result *remoteReusableWorkflow, org, repo, filename, ref string) {
	require.NotNil(t, result)
	assert.Equal(t, org, result.Org)
	assert.Equal(t, repo, result.Repo)
	assert.Equal(t, filename, result.Filename)
	assert.Equal(t, ref, result.Ref)
}
