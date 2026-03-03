package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRemoteReusableWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		uses     string
		expected *remoteReusableWorkflow
	}{
		{
			name: "valid remote workflow",
			uses: "org/repo/.github/workflows/workflow.yml@main",
			expected: &remoteReusableWorkflow{
				Org:      "org",
				Repo:     "repo",
				Filename: "workflow.yml",
				Ref:      "main",
				URL:      "https://github.com",
			},
		},
		{
			name: "valid remote workflow with sha",
			uses: "org/repo/.github/workflows/workflow.yml@abc123def456",
			expected: &remoteReusableWorkflow{
				Org:      "org",
				Repo:     "repo",
				Filename: "workflow.yml",
				Ref:      "abc123def456",
				URL:      "https://github.com",
			},
		},
		{
			name: "valid remote workflow with tag",
			uses: "my-org/my-repo/.github/workflows/my-workflow.yaml@v1.2.3",
			expected: &remoteReusableWorkflow{
				Org:      "my-org",
				Repo:     "my-repo",
				Filename: "my-workflow.yaml",
				Ref:      "v1.2.3",
				URL:      "https://github.com",
			},
		},
		{
			name:     "invalid - local workflow",
			uses:     "./.github/workflows/workflow.yml",
			expected: nil,
		},
		{
			name:     "invalid - missing ref",
			uses:     "org/repo/.github/workflows/workflow.yml",
			expected: nil,
		},
		{
			name:     "invalid - action reference",
			uses:     "org/repo@v1",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := newRemoteReusableWorkflow(tt.uses)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Org, result.Org)
				assert.Equal(t, tt.expected.Repo, result.Repo)
				assert.Equal(t, tt.expected.Filename, result.Filename)
				assert.Equal(t, tt.expected.Ref, result.Ref)
				assert.Equal(t, tt.expected.URL, result.URL)
			}
		})
	}
}

func TestRemoteReusableWorkflowCloneURL(t *testing.T) {
	r := &remoteReusableWorkflow{
		URL:  "https://github.com",
		Org:  "my-org",
		Repo: "my-repo",
	}
	assert.Equal(t, "https://github.com/my-org/my-repo", r.CloneURL())
}

func TestRemoteReusableWorkflowCacheKey(t *testing.T) {
	r := &remoteReusableWorkflow{
		Org:  "my-org",
		Repo: "my-repo",
		Ref:  "main",
	}
	assert.Equal(t, "my-org/my-repo@main", r.cacheKey())
}
