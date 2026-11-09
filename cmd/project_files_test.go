package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadProjectEnvsSkipsSymlinkOutOfWorkdir(t *testing.T) {
	outside := t.TempDir()
	hostFile := filepath.Join(outside, "credentials")
	require.NoError(t, os.WriteFile(hostFile, []byte("TOKEN=host-secret\n"), 0o600))

	workdir := t.TempDir()
	link := filepath.Join(workdir, ".secrets")
	if err := os.Symlink(hostFile, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	assert.True(t, escapesWorkdir(workdir, link))
	secrets := map[string]string{}
	assert.False(t, readProjectEnvs(workdir, link, secrets, true))
	assert.Empty(t, secrets)
}

func TestReadProjectEnvsReadsFilesInsideWorkdir(t *testing.T) {
	workdir := t.TempDir()
	file := filepath.Join(workdir, ".env")
	require.NoError(t, os.WriteFile(file, []byte("A=1\n"), 0o600))
	link := filepath.Join(workdir, ".vars")
	if err := os.Symlink(file, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	envs := map[string]string{}
	assert.True(t, readProjectEnvs(workdir, file, envs, false))
	assert.Equal(t, "1", envs["A"])

	vars := map[string]string{}
	assert.True(t, readProjectEnvs(workdir, link, vars, false))
	assert.Equal(t, "1", vars["A"])
}

func TestReadProjectEnvsTrustsExplicitOutsidePaths(t *testing.T) {
	outside := t.TempDir()
	file := filepath.Join(outside, "my.secrets")
	require.NoError(t, os.WriteFile(file, []byte("TOKEN=mine\n"), 0o600))

	workdir := t.TempDir()
	assert.False(t, escapesWorkdir(workdir, file))
	secrets := map[string]string{}
	assert.True(t, readProjectEnvs(workdir, file, secrets, true))
	assert.Equal(t, "mine", secrets["TOKEN"])
}

func TestReadProjectEnvsMissingFile(t *testing.T) {
	workdir := t.TempDir()
	envs := map[string]string{}
	assert.False(t, readProjectEnvs(workdir, filepath.Join(workdir, ".env"), envs, false))
}
