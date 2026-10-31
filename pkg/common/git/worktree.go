package git

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// ReconstituteWorktree detects whether workdir is a git worktree (i.e. its
// .git entry is a file rather than a directory) and, if so, creates a
// temporary directory that contains the working-tree files together with a
// self-contained .git/ directory that is usable inside a Linux container.
//
// The returned cleanup function must be called when the temporary directory is
// no longer needed.  If workdir is not a worktree the original path is
// returned unchanged together with a no-op cleanup.
func ReconstituteWorktree(_ context.Context, workdir string) (string, func(), error) {
	noop := func() {}

	gitPath := filepath.Join(workdir, ".git")
	fi, err := os.Lstat(gitPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return workdir, noop, nil
		}
		return "", noop, err
	}

	// Regular directory – not a worktree file.
	if fi.IsDir() {
		return workdir, noop, nil
	}

	// .git must be a regular file for a worktree.
	if !fi.Mode().IsRegular() {
		return workdir, noop, nil
	}

	log.Debugf("worktree: detected worktree .git file at %s", gitPath)

	// ------------------------------------------------------------------
	// 1. Parse the .git file to find the gitdir for this worktree.
	// ------------------------------------------------------------------
	gitdir, err := parseGitFile(gitPath, workdir)
	if err != nil {
		return "", noop, err
	}

	// ------------------------------------------------------------------
	// 2. Read <gitdir>/commondir to locate the main repository's git dir.
	// ------------------------------------------------------------------
	commondir, err := readCommondir(gitdir)
	if err != nil {
		return "", noop, err
	}

	// ------------------------------------------------------------------
	// 3. Create a temporary directory.
	// ------------------------------------------------------------------
	tempdir, err := os.MkdirTemp("", "act-worktree-*")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() {
		os.RemoveAll(tempdir)
	}

	// ------------------------------------------------------------------
	// 4. Copy the working tree (skip .git).
	// ------------------------------------------------------------------
	if err := copyWorkingTree(workdir, tempdir); err != nil {
		cleanup()
		return "", noop, err
	}

	// ------------------------------------------------------------------
	// 5. Build tempdir/.git from commondir with worktree overlay.
	// ------------------------------------------------------------------
	destGit := filepath.Join(tempdir, ".git")
	if err := os.MkdirAll(destGit, 0o755); err != nil {
		cleanup()
		return "", noop, err
	}

	// 5a. Copy commondir into tempdir/.git, skipping worktrees/ and *.lock.
	if err := copyCommondir(commondir, destGit); err != nil {
		cleanup()
		return "", noop, err
	}

	// 5b. Overlay worktree-specific files from gitdir.
	worktreeOverlayFiles := []string{"HEAD", "index", "ORIG_HEAD", "FETCH_HEAD"}
	for _, name := range worktreeOverlayFiles {
		src := filepath.Join(gitdir, name)
		dst := filepath.Join(destGit, name)
		if err := copyFileIfExists(src, dst); err != nil {
			cleanup()
			return "", noop, err
		}
	}

	// 5b (cont). Overlay logs/HEAD if present.
	logsHeadSrc := filepath.Join(gitdir, "logs", "HEAD")
	logsHeadDst := filepath.Join(destGit, "logs", "HEAD")
	if err := os.MkdirAll(filepath.Dir(logsHeadDst), 0o755); err != nil {
		cleanup()
		return "", noop, err
	}
	if err := copyFileIfExists(logsHeadSrc, logsHeadDst); err != nil {
		cleanup()
		return "", noop, err
	}

	// 5c. Remove stale commondir pointer if it was copied.
	_ = os.Remove(filepath.Join(destGit, "commondir"))

	log.Debugf("worktree: reconstituted worktree to %s", tempdir)

	return tempdir, cleanup, nil
}

// parseGitFile reads the .git file and returns the gitdir path it contains.
// If the stored path is relative it is resolved relative to workdir.
func parseGitFile(gitFilePath, workdir string) (string, error) {
	data, err := os.ReadFile(gitFilePath)
	if err != nil {
		return "", err
	}
	// Use only the first line — .git files are single-line but be safe.
	raw := strings.TrimSpace(string(data))
	line, _, _ := strings.Cut(raw, "\n")
	const prefix = "gitdir:"
	if !strings.HasPrefix(line, prefix) {
		return "", errors.New("worktree: .git file does not contain a gitdir: line")
	}
	gitdir := strings.TrimSpace(line[len(prefix):])
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Clean(filepath.Join(workdir, gitdir))
	}
	return gitdir, nil
}

// readCommondir reads <gitdir>/commondir and returns the resolved path.
func readCommondir(gitdir string) (string, error) {
	commondirFile := filepath.Join(gitdir, "commondir")
	data, err := os.ReadFile(commondirFile)
	if err != nil {
		// If commondir doesn't exist, the gitdir itself is the common dir.
		if errors.Is(err, os.ErrNotExist) {
			return gitdir, nil
		}
		return "", err
	}
	rel := strings.TrimSpace(string(data))
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel), nil
	}
	return filepath.Clean(filepath.Join(gitdir, rel)), nil
}

// copyWorkingTree copies all working-tree files from src to dst, skipping
// the .git entry at the top level.
func copyWorkingTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip .git at the top level.  When .git is a directory (normal repo)
		// SkipDir skips the subtree.  When .git is a file (worktree) we must
		// return nil — SkipDir on a non-directory skips ALL remaining siblings.
		if rel == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		destPath := filepath.Join(dst, rel)

		typ := d.Type()

		switch {
		case typ.IsDir():
			return os.MkdirAll(destPath, 0o755)

		case typ.IsRegular():
			info, err := d.Info()
			if err != nil {
				return err
			}
			return copyFileWithMode(path, destPath, info.Mode())

		case typ&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, destPath) //nolint:gosec // G122: copying a known worktree, not following untrusted symlinks

		default:
			return nil // ignore pipes, devices, etc.
		}
	})
}

// copyCommondir recursively copies commondir into destGit, skipping the
// worktrees/ subtree and any *.lock files.
func copyCommondir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		// Skip worktrees/ subtree.
		topLevel := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		if topLevel == "worktrees" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip *.lock files.
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".lock") {
			return nil
		}

		destPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return copyFileWithMode(path, destPath, info.Mode())
		}

		return nil
	})
}

// copyFileIfExists copies src to dst if src exists; ENOENT is silently ignored.
func copyFileIfExists(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return copyFileWithMode(src, dst, srcInfo.Mode())
}

// copyFileWithMode copies a regular file preserving its mode bits.
func copyFileWithMode(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
