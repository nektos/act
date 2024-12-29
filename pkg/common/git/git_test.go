package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nektos/act/pkg/common"
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
		{"git@github.com:nektos/act", "GitHub", "nektos/act"},
		{"https://github.com/nektos/act.git", "GitHub", "nektos/act"},
		{"http://github.com/nektos/act.git", "GitHub", "nektos/act"},
		{"https://github.com/nektos/act", "GitHub", "nektos/act"},
		{"http://github.com/nektos/act", "GitHub", "nektos/act"},
		{"git+ssh://git@github.com/owner/repo.git", "GitHub", "owner/repo"},
		{"http://myotherrepo.com/act.git", "", "http://myotherrepo.com/act.git"},
	}

	for _, tt := range slugTests {
		provider, slug, err := findGitSlug(tt.url, "github.com")

		assert.NoError(err)
		assert.Equal(tt.provider, provider)
		assert.Equal(tt.slug, slug)
	}
}

func testDir(t *testing.T) string {
	basedir, err := os.MkdirTemp("", "act-test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(basedir) })
	return basedir
}

func cleanGitHooks(dir string) error {
	hooksDir := filepath.Join(dir, ".git", "hooks")
	files, err := os.ReadDir(hooksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		relName := filepath.Join(hooksDir, f.Name())
		if err := os.Remove(relName); err != nil {
			return err
		}
	}
	return nil
}

func TestFindGitRemoteURL(t *testing.T) {
	assert := assert.New(t)

	basedir := testDir(t)
	gitConfig()
	err := gitCmd("init", basedir)
	assert.NoError(err)
	err = cleanGitHooks(basedir)
	assert.NoError(err)

	remoteURL := "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo-name"
	err = gitCmd("-C", basedir, "remote", "add", "origin", remoteURL)
	assert.NoError(err)

	u, err := findGitRemoteURL(context.Background(), basedir, "origin")
	assert.NoError(err)
	assert.Equal(remoteURL, u)

	remoteURL = "git@github.com/AwesomeOwner/MyAwesomeRepo.git"
	err = gitCmd("-C", basedir, "remote", "add", "upstream", remoteURL)
	assert.NoError(err)
	u, err = findGitRemoteURL(context.Background(), basedir, "upstream")
	assert.NoError(err)
	assert.Equal(remoteURL, u)
}

func TestGitFindRef(t *testing.T) {
	basedir := testDir(t)
	gitConfig()

	for name, tt := range map[string]struct {
		Prepare func(t *testing.T, dir string)
		Assert  func(t *testing.T, ref string, err error)
	}{
		"new_repo": {
			Prepare: func(_ *testing.T, _ string) {},
			Assert: func(t *testing.T, _ string, err error) {
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
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(basedir, name)
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, gitCmd("-C", dir, "init", "--initial-branch=master"))
			require.NoError(t, cleanGitHooks(dir))
			tt.Prepare(t, dir)
			ref, err := FindGitRef(context.Background(), dir)
			tt.Assert(t, ref, err)
		})
	}
}

func TestGitCloneExecutor(t *testing.T) {
	for name, tt := range map[string]struct {
		Err      error
		URL, Ref string
	}{
		"tag": {
			Err: nil,
			URL: "https://github.com/actions/checkout",
			Ref: "v2",
		},
		"branch": {
			Err: nil,
			URL: "https://github.com/anchore/scan-action",
			Ref: "act-fails",
		},
		"sha": {
			Err: nil,
			URL: "https://github.com/actions/checkout",
			Ref: "5a4ac9002d0be2fb38bd78e4b4dbde5606d7042f", // v2
		},
		"short-sha": {
			Err: &Error{ErrShortRef, "5a4ac9002d0be2fb38bd78e4b4dbde5606d7042f"},
			URL: "https://github.com/actions/checkout",
			Ref: "5a4ac90", // v2
		},
	} {
		t.Run(name, func(t *testing.T) {
			clone := NewGitCloneExecutor(NewGitCloneExecutorInput{
				URL: tt.URL,
				Ref: tt.Ref,
				Dir: testDir(t),
			})

			err := clone(context.Background())
			if tt.Err != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.Err, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func gitConfig() {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		var err error
		if err = gitCmd("config", "--global", "user.email", "test@test.com"); err != nil {
			log.Error(err)
		}
		if err = gitCmd("config", "--global", "user.name", "Unit Test"); err != nil {
			log.Error(err)
		}
	}
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

func TestCloneIfRequired(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	t.Run("clone", func(t *testing.T) {
		repo, err := CloneIfRequired(ctx, "refs/heads/main", NewGitCloneExecutorInput{
			URL: "https://github.com/actions/checkout",
			Dir: tempDir,
		}, common.Logger(ctx))
		assert.NoError(t, err)
		assert.NotNil(t, repo)
	})

	t.Run("clone different remote", func(t *testing.T) {
		repo, err := CloneIfRequired(ctx, "refs/heads/main", NewGitCloneExecutorInput{
			URL: "https://github.com/actions/setup-go",
			Dir: tempDir,
		}, common.Logger(ctx))
		require.NoError(t, err)
		require.NotNil(t, repo)

		remote, err := repo.Remote("origin")
		require.NoError(t, err)
		require.Len(t, remote.Config().URLs, 1)
		assert.Equal(t, "https://github.com/actions/setup-go", remote.Config().URLs[0])
	})
}
