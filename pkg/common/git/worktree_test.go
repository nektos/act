package git

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile creates all necessary parent directories and writes content to path.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestReconstituteWorktree_NotAWorktree(t *testing.T) {
	dir := t.TempDir()
	// .git is a directory – not a worktree file
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")

	result, cleanup, err := ReconstituteWorktree(context.Background(), dir)
	require.NoError(t, err)
	defer cleanup()

	assert.Equal(t, dir, result)
}

func TestReconstituteWorktree_NoGit(t *testing.T) {
	dir := t.TempDir()
	// No .git at all

	result, cleanup, err := ReconstituteWorktree(context.Background(), dir)
	require.NoError(t, err)
	defer cleanup()

	assert.Equal(t, dir, result)
}

func TestReconstituteWorktree_FakeWorktree(t *testing.T) {
	base := t.TempDir()

	// Build main repository structure:
	//   base/main/.git/HEAD
	//   base/main/.git/config
	//   base/main/.git/refs/heads/main  (a ref file)
	//   base/main/.git/worktrees/wt1/HEAD
	//   base/main/.git/worktrees/wt1/commondir  (contains "../..")
	//   base/main/.git/worktrees/wt1/gitdir      (path back to wt1/.git)

	mainGit := filepath.Join(base, "main", ".git")
	wt1GitDir := filepath.Join(mainGit, "worktrees", "wt1")

	writeFile(t, filepath.Join(mainGit, "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(mainGit, "config"), "[core]\n\trepositoryformatversion = 0\n")
	writeFile(t, filepath.Join(mainGit, "refs", "heads", "main"), "abc1234\n")
	writeFile(t, filepath.Join(wt1GitDir, "HEAD"), "ref: refs/heads/feature\n")
	writeFile(t, filepath.Join(wt1GitDir, "commondir"), "../..\n")
	// gitdir file in the wt1GitDir just needs to exist; its contents point
	// back to the worktree directory.
	wt1Dir := filepath.Join(base, "wt1")
	writeFile(t, filepath.Join(wt1GitDir, "gitdir"), filepath.Join(wt1Dir, ".git")+"\n")

	// Build worktree directory:
	//   base/wt1/.git  (file pointing to main/.git/worktrees/wt1)
	//   base/wt1/somefile.txt
	writeFile(t, filepath.Join(wt1Dir, ".git"), "gitdir: "+wt1GitDir+"\n")
	writeFile(t, filepath.Join(wt1Dir, "somefile.txt"), "hello\n")

	result, cleanup, err := ReconstituteWorktree(context.Background(), wt1Dir)
	require.NoError(t, err)
	defer cleanup()

	assert.NotEqual(t, wt1Dir, result, "should return a new tempdir, not the original")

	// .git should now be a directory in the result
	resultGitStat, err := os.Lstat(filepath.Join(result, ".git"))
	require.NoError(t, err)
	assert.True(t, resultGitStat.IsDir(), ".git in reconstituted dir should be a directory")

	// HEAD should be the worktree HEAD (overlaid from wt1GitDir), not main HEAD
	headContent, err := os.ReadFile(filepath.Join(result, ".git", "HEAD"))
	require.NoError(t, err)
	assert.Equal(t, "ref: refs/heads/feature\n", string(headContent))

	// config from commondir should be present
	configContent, err := os.ReadFile(filepath.Join(result, ".git", "config"))
	require.NoError(t, err)
	assert.Contains(t, string(configContent), "repositoryformatversion")

	// somefile.txt should have been copied
	somefileContent, err := os.ReadFile(filepath.Join(result, "somefile.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(somefileContent))

	// worktrees/ subtree should NOT be present in the reconstituted .git
	_, err = os.Lstat(filepath.Join(result, ".git", "worktrees"))
	assert.True(t, os.IsNotExist(err), "worktrees/ subtree should not be copied")

	// commondir pointer should have been removed
	_, err = os.Lstat(filepath.Join(result, ".git", "commondir"))
	assert.True(t, os.IsNotExist(err), "stale commondir file should be removed")
}

func TestReconstituteWorktree_PreservesSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows – requires elevated privileges")
	}

	dir := t.TempDir()
	// Make a plain directory (not a worktree) with a symlink in it.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(dir, "realfile.txt"), "contents\n")
	require.NoError(t, os.Symlink("realfile.txt", filepath.Join(dir, "link.txt")))

	// For this test we manually build a fake worktree so the reconstitution
	// is triggered and we can assert symlinks are preserved.
	base := t.TempDir()
	mainGit := filepath.Join(base, "main", ".git")
	wt1GitDir := filepath.Join(mainGit, "worktrees", "wt1")

	writeFile(t, filepath.Join(mainGit, "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(mainGit, "config"), "[core]\n\trepositoryformatversion = 0\n")
	writeFile(t, filepath.Join(wt1GitDir, "HEAD"), "ref: refs/heads/feature\n")
	writeFile(t, filepath.Join(wt1GitDir, "commondir"), "../..\n")

	wt1Dir := filepath.Join(base, "wt1")
	require.NoError(t, os.MkdirAll(wt1Dir, 0o755))
	writeFile(t, filepath.Join(wt1Dir, ".git"), "gitdir: "+wt1GitDir+"\n")
	writeFile(t, filepath.Join(wt1Dir, "realfile.txt"), "contents\n")
	require.NoError(t, os.Symlink("realfile.txt", filepath.Join(wt1Dir, "link.txt")))

	result, cleanup, err := ReconstituteWorktree(context.Background(), wt1Dir)
	require.NoError(t, err)
	defer cleanup()

	// Verify the symlink target is preserved
	target, err := os.Readlink(filepath.Join(result, "link.txt"))
	require.NoError(t, err)
	assert.Equal(t, "realfile.txt", target)
}

func TestReconstituteWorktree_RelativeGitdirInCommondir(t *testing.T) {
	base := t.TempDir()

	// Layout:
	//   base/repo/.git/        (main git dir)
	//   base/repo/.git/worktrees/mywt/HEAD
	//   base/repo/.git/worktrees/mywt/commondir  -> "../.."  (relative: resolves to base/repo/.git)
	//   base/wt/.git           (worktree file)
	//   base/wt/file.go

	repoGit := filepath.Join(base, "repo", ".git")
	wtGitDir := filepath.Join(repoGit, "worktrees", "mywt")

	writeFile(t, filepath.Join(repoGit, "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(repoGit, "config"), "[core]\n\trepositoryformatversion = 0\n")
	writeFile(t, filepath.Join(wtGitDir, "HEAD"), "ref: refs/heads/my-feature\n")
	// commondir is relative: "../.." from base/repo/.git/worktrees/mywt resolves to base/repo/.git
	writeFile(t, filepath.Join(wtGitDir, "commondir"), "../..\n")

	wtDir := filepath.Join(base, "wt")
	writeFile(t, filepath.Join(wtDir, ".git"), "gitdir: "+wtGitDir+"\n")
	writeFile(t, filepath.Join(wtDir, "file.go"), "package main\n")

	result, cleanup, err := ReconstituteWorktree(context.Background(), wtDir)
	require.NoError(t, err)
	defer cleanup()

	assert.NotEqual(t, wtDir, result)

	// HEAD should be the worktree HEAD
	headContent, err := os.ReadFile(filepath.Join(result, ".git", "HEAD"))
	require.NoError(t, err)
	assert.Equal(t, "ref: refs/heads/my-feature\n", string(headContent))

	// config from the main repo should be present
	configContent, err := os.ReadFile(filepath.Join(result, ".git", "config"))
	require.NoError(t, err)
	assert.Contains(t, string(configContent), "repositoryformatversion")
}
