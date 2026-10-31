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
		// secretlint-disable-next-line @secretlint/secretlint-rule-basicauth
		{"https://x-access-token:abc@github.com/owner/repo.git", "GitHub", "owner/repo"},
		{"ssh://git@github.com/owner/repo", "GitHub", "owner/repo"},
		{"https://github.com:443/owner/repo.git", "GitHub", "owner/repo"},
		{"git://github.com/owner/repo.git", "GitHub", "owner/repo"},
		{"ssh://git@github.com:22/owner/repo.git", "GitHub", "owner/repo"},
		{"ssh://git@ssh.github.com:443/owner/repo.git", "GitHub", "owner/repo"},
		{"git@ssh.github.com:owner/repo.git", "GitHub", "owner/repo"},
		{"https://ssh.github.com/owner/repo", "", "https://ssh.github.com/owner/repo"},
		{"https://github.com:evil.test/owner/repo", "", "https://github.com:evil.test/owner/repo"},
		{"ssh://git@github.com:22.evil.test/owner/repo", "", "ssh://git@github.com:22.evil.test/owner/repo"},
		{"git://evil.test/github.com/owner/repo", "", "git://evil.test/github.com/owner/repo"},
		{"https://evil.test?x=@github.com:443/owner/repo", "", "https://evil.test?x=@github.com:443/owner/repo"},
		{"https://evil.test#@github.com/owner/repo", "", "https://evil.test#@github.com/owner/repo"},
		{"ssh://evil.test?x=@github.com/owner/repo", "", "ssh://evil.test?x=@github.com/owner/repo"},
		{"git://evil.test?x=@github.com/owner/repo", "", "git://evil.test?x=@github.com/owner/repo"},
		{"git://evil.test#@github.com/owner/repo", "", "git://evil.test#@github.com/owner/repo"},
		{"evil.test?x=@github.com:owner/repo", "", "evil.test?x=@github.com:owner/repo"},
		{"https://evilgithub.com/owner/repo", "", "https://evilgithub.com/owner/repo"},
		{"https://github.com.evil.test/owner/repo", "", "https://github.com.evil.test/owner/repo"},
		{"https://evil.test/github.com/owner/repo", "", "https://evil.test/github.com/owner/repo"},
		{"https://githubXcom/owner/repo", "", "https://githubXcom/owner/repo"},
		{"git@evil.test:github.com/owner/repo", "", "git@evil.test:github.com/owner/repo"},
		{"git@github.com.evil.test:owner/repo", "", "git@github.com.evil.test:owner/repo"},
		{"https://git-codecommit.us-east-1Xamazonaws.com/v1/repos/r", "", "https://git-codecommit.us-east-1Xamazonaws.com/v1/repos/r"},
		{"https://git-codecommit.us-east-1.amazonaws.com.evil.test/v1/repos/r", "", "https://git-codecommit.us-east-1.amazonaws.com.evil.test/v1/repos/r"},
	}

	for _, tt := range slugTests {
		provider, slug, err := findGitSlug(tt.url, "github.com")

		assert.NoError(err)
		assert.Equal(tt.provider, provider)
		assert.Equal(tt.slug, slug)
	}
}

func TestFindGitSlugEnterprise(t *testing.T) {
	for url, want := range map[string]string{
		"https://ghe.example.com/owner/repo.git":     "owner/repo",
		"git@ghe.example.com:owner/repo.git":         "owner/repo",
		"https://ghe.example.com:8443/owner/repo":    "owner/repo",
		"ssh://git@ghe.example.com:2222/owner/repo":  "owner/repo",
		"https://gheXexample.com/owner/repo":         "",
		"https://ghe.example.com.evil.test/owner/re": "",
	} {
		provider, slug, err := findGitSlug(url, "ghe.example.com")
		assert.NoError(t, err)
		assertEnterpriseSlug(t, url, want, provider, slug)
	}

	// The instance can include the HTTPS port; SSH remotes use their own port.
	for url, want := range map[string]string{
		"https://ghe.example.com:8443/owner/repo":       "owner/repo",
		"ssh://git@ghe.example.com:2222/owner/repo":     "owner/repo",
		"ssh://git@ghe.example.com/owner/repo.git":      "owner/repo",
		"git@ghe.example.com:owner/repo.git":            "owner/repo",
		"https://ghe.example.com:8443.evil.test/o/r":    "",
		"https://ghe.example.com:9443/owner/repo":       "",
		"ssh://git@ghe.example.com.evil.test:2222/o/r":  "",
		"https://evil.test?x=@ghe.example.com:8443/o/r": "",
	} {
		provider, slug, err := findGitSlug(url, "ghe.example.com:8443")
		assert.NoError(t, err)
		assertEnterpriseSlug(t, url, want, provider, slug)
	}

	// IPv6 literals keep their brackets after the HTTPS port is removed.
	for url, want := range map[string]string{
		"https://[2001:db8::1]:8443/owner/repo":        "owner/repo",
		"ssh://git@[2001:db8::1]:8443/owner/repo":      "owner/repo",
		"ssh://git@[2001:db8::1]:2222/owner/repo":      "owner/repo",
		"ssh://git@[2001:db8::1]/owner/repo.git":       "owner/repo",
		"git://[2001:db8::1]/owner/repo.git":           "owner/repo",
		"git@[2001:db8::1]:owner/repo.git":             "owner/repo",
		"https://[2001:db8::1]:2222/owner/repo":        "",
		"ssh://git@[2001:db8::12]:8443/owner/repo":     "",
		"ssh://git@[2001:db8::1].evil.test/owner/repo": "",
		"ssh://git@[2001:db8::1]:8443.evil/owner/repo": "",
		"https://evil.test?x=@[2001:db8::1]:8443/o/r":  "",
		"ssh://git@2001:db8::1:8443/owner/repo":        "",
	} {
		provider, slug, err := findGitSlug(url, "[2001:db8::1]:8443")
		assert.NoError(t, err)
		assertEnterpriseSlug(t, url, want, provider, slug)
	}

	// An instance with no port is already bracketed; SplitHostPort must not
	// strip those brackets on the error path.
	for url, want := range map[string]string{
		"ssh://git@[2001:db8::1]/owner/repo":      "owner/repo",
		"https://[2001:db8::1]/owner/repo.git":    "owner/repo",
		"ssh://git@[2001:db8::1]:2222/owner/repo": "owner/repo",
		"ssh://git@[2001:db8::2]/owner/repo":      "",
	} {
		provider, slug, err := findGitSlug(url, "[2001:db8::1]")
		assert.NoError(t, err)
		assertEnterpriseSlug(t, url, want, provider, slug)
	}
}

func assertEnterpriseSlug(t *testing.T, url, want, provider, slug string) {
	t.Helper()
	if want == "" {
		assert.Equal(t, "", provider, url)
		assert.Equal(t, url, slug)
	} else {
		assert.Equal(t, "GitHubEnterprise", provider, url)
		assert.Equal(t, want, slug, url)
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
