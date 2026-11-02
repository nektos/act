package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nektos/act/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"
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

func writeUTF16LE(t *testing.T, name, content string) {
	t.Helper()
	encoded, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().String(content)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(name), 0o755))
	require.NoError(t, os.WriteFile(name, []byte(encoded), 0o600))
}

// Windows PowerShell 5.1 writes `echo x >> $env:GITHUB_ENV` as UTF-16LE with a BOM.
func TestGithubEnvFileUTF16(t *testing.T) {
	actPath := t.TempDir()
	writeUTF16LE(t, filepath.Join(actPath, "workflow", "envs.txt"), "FOO=bar\r\nMULTI<<EOF\r\nline1\r\nline2\r\nEOF\r\n")

	rc := &RunContext{
		Config:       &Config{},
		JobContainer: &container.HostEnvironment{ActPath: actPath},
	}
	require.NoError(t, processRunnerGithubEnvFileCommand(context.Background(), "workflow/envs.txt", rc))
	assert.Equal(t, "bar", rc.Env["FOO"])
	assert.Equal(t, "line1\nline2", rc.Env["MULTI"])
}

// Reading a UTF-16LE GITHUB_PATH as UTF-8 put NULs into PATH (nektos/act#2395).
func TestGithubPathFileUTF16(t *testing.T) {
	actPath := t.TempDir()
	writeUTF16LE(t, filepath.Join(actPath, "workflow", "paths.txt"), "C:\\cygwin\\bin\r\n")

	rc := &RunContext{
		Config:       &Config{},
		JobContainer: &container.HostEnvironment{ActPath: actPath},
	}
	require.NoError(t, rc.UpdateExtraPath(context.Background(), filepath.Join(actPath, "workflow", "paths.txt")))
	assert.Equal(t, []string{"C:\\cygwin\\bin"}, rc.ExtraPath)
}
