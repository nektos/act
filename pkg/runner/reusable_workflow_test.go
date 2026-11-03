package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReusableWorkflowPathsStayInRepository(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "a", "repo")
	workflows := filepath.Join(repo, ".github", "workflows")
	require.NoError(t, os.MkdirAll(workflows, 0o755))
	body := []byte(`name: called
on:
  workflow_call:
jobs:
  called:
    if: "false"
    runs-on: ubuntu-latest
    steps:
      - run: "true"
`)
	require.NoError(t, os.WriteFile(filepath.Join(workflows, "called.yml"), body, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(base, "outside.yml"), body, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(base, "a", "outside.yml"), body, 0o600))

	rc := &RunContext{
		Name: "call",
		Config: &Config{
			Workdir: repo,
			Env:     map[string]string{},
		},
		Run: &model.Run{
			JobID: "call",
			Workflow: &model.Workflow{
				Name: "caller",
				Jobs: map[string]*model.Job{
					"call": {},
				},
			},
		},
	}

	tests := []struct {
		name    string
		uses    string
		remote  bool
		wantErr string
	}{
		{
			name:    "local uses escapes",
			uses:    "./../../outside.yml",
			wantErr: "resolves outside",
		},
		{
			name:    "remote filename escapes",
			uses:    "org/repo/.github/workflows/../../../outside.yml@ref",
			remote:  true,
			wantErr: "resolves outside",
		},
		{
			name: "local workflow",
			uses: "./.github/workflows/called.yml",
		},
		{
			name:   "remote workflow",
			uses:   "org/repo/.github/workflows/called.yml@ref",
			remote: true,
		},
		{
			name: "local parent stays inside",
			uses: "./.github/workflows/../workflows/called.yml",
		},
		{
			name:   "remote parent stays inside",
			uses:   "org/repo/.github/workflows/../workflows/called.yml@ref",
			remote: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := tt.uses
			if tt.remote {
				remote := newRemoteReusableWorkflow(tt.uses)
				require.NotNil(t, remote)
				workflow = fmt.Sprintf("./.github/workflows/%s", remote.Filename)
			}
			err := newReusableWorkflowExecutor(rc, repo, workflow)(context.Background())
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}
