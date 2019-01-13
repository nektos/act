package common

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindGitSlug(t *testing.T) {
	assert := assert.New(t)

	var slugTests = []struct {
		url      string // input
		provider string // expected result
		slug     string // expected result
	}{
		{"https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo-name", "CodeCommit", "my-repo-name"},
		{"ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/my-repo", "CodeCommit", "my-repo"},
		{"git@github.com:nektos/act.git", "GitHub", "nektos/act"},
		{"https://github.com/nektos/act.git", "GitHub", "nektos/act"},
		{"http://github.com/nektos/act.git", "GitHub", "nektos/act"},
		{"http://myotherrepo.com/act.git", "", "http://myotherrepo.com/act.git"},
	}

	for _, tt := range slugTests {
		provider, slug, err := findGitSlug(tt.url)

		assert.Nil(err)
		assert.Equal(tt.provider, provider)
		assert.Equal(tt.slug, slug)
	}

}

func TestFindGitRemoteURL(t *testing.T) {
	assert := assert.New(t)

	basedir, err := ioutil.TempDir("", "act-test")
	defer os.RemoveAll(basedir)

	assert.Nil(err)

	err = gitCmd("init", basedir)
	assert.Nil(err)

	remoteURL := "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo-name"
	err = gitCmd("config", "-f", fmt.Sprintf("%s/.git/config", basedir), "--add", "remote.origin.url", remoteURL)
	assert.Nil(err)

	u, err := findGitRemoteURL(basedir)
	assert.Nil(err)
	assert.Equal(remoteURL, u)
}

func gitCmd(args ...string) error {
	var stdout bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = ioutil.Discard

	err := cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok {
		if waitStatus, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return fmt.Errorf("Exit error %d", waitStatus.ExitStatus())
		}
		return exitError
	}
	return nil
}
