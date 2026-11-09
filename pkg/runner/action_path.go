package runner

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// joinUnder joins rel onto root. rel is slash-separated and a leading slash
// does not make it absolute. The result stays lexically inside root.
func joinUnder(root, rel string) (string, error) {
	root = filepath.Clean(root)
	rel = strings.TrimLeft(strings.ReplaceAll(rel, "\\", "/"), "/")
	if rel == "" || rel == "." {
		return root, nil
	}
	rel = filepath.FromSlash(path.Clean(rel))
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("path %q resolves outside of %q", rel, root)
	}
	target := filepath.Join(root, rel)
	if err := lexicalInside(root, target); err != nil {
		return "", err
	}
	return target, nil
}

func lexicalInside(root, target string) error {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return err
	}
	if rel != "." && !filepath.IsLocal(rel) {
		return fmt.Errorf("path %q resolves outside of %q", target, root)
	}
	return nil
}

// resolveLocalActionDir returns the host directory of a local action
// (uses: ./...). The directory stays inside workdir.
func resolveLocalActionDir(workdir, uses string) (string, error) {
	return joinUnder(workdir, strings.TrimPrefix(uses, "./"))
}

// localActionWorkspace is the directory a local action is executed from.
// A job container uses the workdir. The host executor copies that workdir
// and the action path is relative to the copy, so the copy is the root.
func localActionWorkspace(rc *RunContext) string {
	if rc == nil || rc.Config == nil {
		return ""
	}
	if rc.JobContainer != nil {
		return rc.JobContainer.ToContainerPath(rc.Config.Workdir)
	}
	return rc.Config.Workdir
}

// rootedWriter writes filename, which must stay inside root, without
// following a symlink out of root.
func rootedWriter(root string) fileWriter {
	return func(filename string, data []byte, perm fs.FileMode) error {
		if err := lexicalInside(root, filename); err != nil {
			return fmt.Errorf("refusing to write %q outside of %q", filename, root)
		}
		rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(filename))
		if err != nil {
			return err
		}
		r, err := os.OpenRoot(root)
		if err != nil {
			return err
		}
		defer r.Close()
		return r.WriteFile(rel, data, perm)
	}
}

// resolveDockerContext splits runs.image into a context directory and a
// Dockerfile name. For a local action, actionPath is the absolute action
// directory and the returned context directory is absolute and inside it.
// Otherwise actionPath is the repository-relative action directory and the
// returned context directory is repository-relative (empty means the
// repository root).
func resolveDockerContext(actionPath, image string, local bool) (string, string, error) {
	relImage := strings.TrimLeft(strings.ReplaceAll(image, "\\", "/"), "/")
	joined := path.Join(actionPath, relImage)
	if local {
		root := path.Clean(actionPath)
		joined = path.Clean(joined)
		dir, file := path.Split(joined)
		dir = path.Clean(dir)
		if file == "" || !slashWithin(root, dir) {
			return "", "", fmt.Errorf("docker build context %q resolves outside of the action directory", image)
		}
		return dir, file, nil
	}
	if !isLocalActionPath(joined) {
		return "", "", fmt.Errorf("docker build context %q resolves outside of the action checkout", image)
	}
	dir, file := path.Split(joined)
	if file == "" {
		return "", "", fmt.Errorf("docker build context %q resolves outside of the action checkout", image)
	}
	relDir := strings.TrimSuffix(dir, "/")
	if relDir != "" && !isLocalActionPath(relDir) {
		return "", "", fmt.Errorf("docker build context %q resolves outside of the action checkout", image)
	}
	return dir, file, nil
}

func slashWithin(root, target string) bool {
	root = path.Clean(root)
	target = path.Clean(target)
	if target == root {
		return true
	}
	if root == "/" {
		return strings.HasPrefix(target, "/")
	}
	return strings.HasPrefix(target, root+"/")
}

// rejectHostContextEscape allows full only when it stays inside root, including
// through symlinks. A root that is not a host directory is ignored so container
// paths on another volume are left to the lexical action-directory check.
func rejectHostContextEscape(root, full string) error {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(full))
	if err != nil {
		return nil
	}
	if rel != "." && !filepath.IsLocal(rel) {
		return fmt.Errorf("docker build context %q resolves outside of %q", full, root)
	}
	info, statErr := os.Stat(filepath.Clean(root))
	if statErr != nil || !info.IsDir() {
		return nil
	}
	rel = strings.TrimSuffix(filepath.ToSlash(rel), "/")
	if rel == "" || rel == "." {
		return nil
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer r.Close()
	if _, err := r.Stat(filepath.FromSlash(rel)); err != nil {
		return fmt.Errorf("docker build context %q resolves outside of %q: %w", rel, root, err)
	}
	return nil
}
