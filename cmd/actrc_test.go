package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFlagSets(t *testing.T) []*pflag.FlagSet {
	t.Helper()
	cmd := createRootCommand(t.Context(), &Input{}, "test")
	return []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags()}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestProjectActrcKeepsOnlyAllowedFlags(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user-actrc")
	project := filepath.Join(dir, "project-actrc")
	writeFile(t, user, "--container-daemon-socket -\n-P ubuntu-latest=user/image\n")
	writeFile(t, project, `-P ubuntu-latest=catthehacker/ubuntu:act-latest
-P self=-self-hosted
-Pother=-self-hosted
--container-daemon-socket /var/run/docker.sock
--privileged
--container-options --privileged
--container-options-policy allow
--local-repository org/repo@v1=/
--artifact-server-path /tmp/x
--secret GITHUB_TOKEN
-s AWS_SECRET_ACCESS_KEY
--env HOME
--env FOO=bar
--var-file ../../.aws/credentials
--secret-file .secrets
--action-cache-path /
--trust-project-actrc
--network host
--env=TOKEN=$GITHUB_TOKEN
-W .github/workflows/ci.yml
--reuse
`)
	t.Setenv("GITHUB_TOKEN", "host-token")

	got := actrcArgs(testFlagSets(t), []string{user}, project, []string{"-j", "build"})
	assert.Equal(t, []string{
		"--container-daemon-socket", "-",
		"-P", "ubuntu-latest=user/image",
		"-P", "ubuntu-latest=catthehacker/ubuntu:act-latest",
		"--env", "FOO=bar",
		"--secret-file", ".secrets",
		"--env=TOKEN=$GITHUB_TOKEN",
		"-W", ".github/workflows/ci.yml",
		"--reuse",
		"-j", "build",
	}, got)
}

func TestProjectActrcTrusted(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user-actrc")
	project := filepath.Join(dir, "project-actrc")
	writeFile(t, user, "--trust-project-actrc\n")
	writeFile(t, project, "--privileged\n--env TOKEN=$ACT_TEST_VALUE\n")
	t.Setenv("ACT_TEST_VALUE", "expanded")

	got := actrcArgs(testFlagSets(t), []string{user}, project, nil)
	assert.Equal(t, []string{"--trust-project-actrc", "--privileged", "--env", "TOKEN=expanded"}, got)

	got = actrcArgs(testFlagSets(t), []string{user}, project, []string{"--trust-project-actrc=false"})
	assert.Equal(t, []string{"--trust-project-actrc", "--env", "TOKEN=$ACT_TEST_VALUE", "--trust-project-actrc=false"}, got)

	got = actrcArgs(testFlagSets(t), nil, project, []string{"--trust-project-actrc"})
	assert.Equal(t, []string{"--privileged", "--env", "TOKEN=expanded", "--trust-project-actrc"}, got)
}

func TestProjectActrcInHomeDirectoryIsUserConfig(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".actrc")
	writeFile(t, home, "--privileged\n")
	t.Chdir(dir)

	got := actrcArgs(testFlagSets(t), []string{home}, filepath.Join(".", ".actrc"), nil)
	assert.Equal(t, []string{"--privileged"}, got)
}

func TestProjectActrcRejectsUnknownShorthand(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project-actrc")
	writeFile(t, project, "-Z value\n-\n-C /\n")
	assert.Empty(t, actrcArgs(testFlagSets(t), nil, project, nil))
}
