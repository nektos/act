package cmd

import (
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// escapesWorkdir reports whether a file inside workdir resolves, through
// symlinks, to a location outside workdir. Files the user names outside the
// workdir are trusted as given.
func escapesWorkdir(workdir, path string) bool {
	if path == "" {
		return false
	}
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if rel, err := filepath.Rel(absWorkdir, absPath); err != nil || !filepath.IsLocal(rel) {
		return false
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return false
	}
	realWorkdir, err := filepath.EvalSymlinks(absWorkdir)
	if err != nil {
		return true
	}
	rel, err := filepath.Rel(realWorkdir, realPath)
	return err != nil || !filepath.IsLocal(rel)
}

func readProjectEnvs(workdir, path string, envs map[string]string, caseInsensitive bool) bool {
	if escapesWorkdir(workdir, path) {
		log.Warnf("Ignoring %s: it links to a file outside %s", path, workdir)
		return false
	}
	return readEnvsEx(path, envs, caseInsensitive)
}
