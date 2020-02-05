package common

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{"https://github.com/nektos/act", "GitHub", "nektos/act"},
		{"http://github.com/nektos/act", "GitHub", "nektos/act"},
		{"git+ssh://git@github.com/owner/repo.git", "GitHub", "owner/repo"},
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

  gitConfig()
	err = gitCmd("init", basedir)
	assert.Nil(err)

	remoteURL := "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo-name"
	err = gitCmd("config", "-f", fmt.Sprintf("%s/.git/config", basedir), "--add", "remote.origin.url", remoteURL)
	assert.Nil(err)

	u, err := findGitRemoteURL(basedir)
	assert.Nil(err)
	assert.Equal(remoteURL, u)
}

func TestGitFindRef(t *testing.T) {
	basedir, err := ioutil.TempDir("", "act-test")
	defer os.RemoveAll(basedir)
	assert.NoError(t, err)

  gitConfig()

	for name, tt := range map[string]struct {
		Prepare func(t *testing.T, dir string)
		Assert  func(t *testing.T, ref string, err error)
	}{
		"new_repo": {
			Prepare: func(t *testing.T, dir string) {},
			Assert: func(t *testing.T, ref string, err error) {
				require.Error(t, err)
			},
		},
		"new_repo_with_commit": {
			Prepare: func(t *testing.T, dir string) {
				require.NoError(t, gitCmd("-C", dir, "commit", "--allow-empty", "-m", "msg"))
			},
			Assert: func(t *testing.T, ref string, err error) {
				require.NoError(t, err)
				require.Equal(t, "refs/heads/master", ref)
			},
		},
		"current_head_is_tag": {
			Prepare: func(t *testing.T, dir string) {
				require.NoError(t, gitCmd("-C", dir, "commit", "--allow-empty", "-m", "commit msg"))
				require.NoError(t, gitCmd("-C", dir, "tag", "v1.2.3"))
				require.NoError(t, gitCmd("-C", dir, "checkout", "v1.2.3"))
			},
			Assert: func(t *testing.T, ref string, err error) {
				require.NoError(t, err)
				require.Equal(t, "refs/tags/v1.2.3", ref)
			},
		},
		"current_head_is_same_as_tag": {
			Prepare: func(t *testing.T, dir string) {
				require.NoError(t, gitCmd("-C", dir, "commit", "--allow-empty", "-m", "1.4.2 release"))
				require.NoError(t, gitCmd("-C", dir, "tag", "v1.4.2"))
			},
			Assert: func(t *testing.T, ref string, err error) {
				require.NoError(t, err)
				require.Equal(t, "refs/tags/v1.4.2", ref)
			},
		},
		"current_head_is_not_tag": {
			Prepare: func(t *testing.T, dir string) {
				require.NoError(t, gitCmd("-C", dir, "commit", "--allow-empty", "-m", "msg"))
				require.NoError(t, gitCmd("-C", dir, "tag", "v1.4.2"))
				require.NoError(t, gitCmd("-C", dir, "commit", "--allow-empty", "-m", "msg2"))
			},
			Assert: func(t *testing.T, ref string, err error) {
				require.NoError(t, err)
				require.Equal(t, "refs/heads/master", ref)
			},
		},
		"current_head_is_another_branch": {
			Prepare: func(t *testing.T, dir string) {
				require.NoError(t, gitCmd("-C", dir, "checkout", "-b", "mybranch"))
				require.NoError(t, gitCmd("-C", dir, "commit", "--allow-empty", "-m", "msg"))
			},
			Assert: func(t *testing.T, ref string, err error) {
				require.NoError(t, err)
				require.Equal(t, "refs/heads/mybranch", ref)
			},
		},
	} {
		tt := tt
		name := name
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(basedir, name)
			require.NoError(t, os.MkdirAll(dir, 0755))
			require.NoError(t, gitCmd("-C", dir, "init"))
			tt.Prepare(t, dir)
			ref, err := FindGitRef(dir)
			tt.Assert(t, ref, err)
		})
	}
}

func gitConfig() {
	_ = gitCmd("config","--global","user.email","test@test.com")
	_ = gitCmd("config","--global","user.name","Unit Test")
}

func gitCmd(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok {
		if waitStatus, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return fmt.Errorf("Exit error %d", waitStatus.ExitStatus())
		}
		return exitError
	}
	return nil
}
