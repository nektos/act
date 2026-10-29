package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nektos/act/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubEnvFileRejectsNodeOptions(t *testing.T) {
	actPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(actPath, "workflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(actPath, "workflow", "envs.txt"),
		[]byte("NODE_OPTIONS=--require /tmp/evil.js\nnode_options=--inspect\nFOO=bar\n"), 0o600))

	rc := &RunContext{
		Config:       &Config{},
		JobContainer: &container.HostEnvironment{ActPath: actPath},
	}
	err := processRunnerGithubEnvFileCommand(context.Background(), "workflow/envs.txt", rc)
	assert.ErrorContains(t, err, "NODE_OPTIONS")
	assert.ErrorContains(t, err, "node_options")
	assert.Equal(t, "bar", rc.Env["FOO"])
	_, ok := rc.Env["NODE_OPTIONS"]
	assert.False(t, ok)
	_, ok = rc.Env["node_options"]
	assert.False(t, ok)
}

func TestSetEnvCommandRejectsNodeOptions(t *testing.T) {
	rc := &RunContext{Config: &Config{}, Env: map[string]string{"ACTIONS_ALLOW_UNSECURE_COMMANDS": "true"}}
	handler := rc.commandHandler(context.Background())
	handler("::set-env name=NODE_OPTIONS::--require /tmp/evil.js\n")
	handler("::set-env name=FOO::bar\n")
	_, ok := rc.Env["NODE_OPTIONS"]
	assert.False(t, ok)
	assert.Equal(t, "bar", rc.Env["FOO"])
}
