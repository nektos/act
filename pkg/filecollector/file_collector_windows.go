package filecollector

import (
	"io/fs"
	"os"
)

func prepareRootBackupRemoval(root *os.Root, name string, mode fs.FileMode) error {
	// Root.Chmod on Windows opens the entry relative to root with
	// FILE_FLAG_OPEN_REPARSE_POINT, so this clears the read-only attribute on
	// the backup itself without following a leaf symlink.
	return root.Chmod(name, mode.Perm()|0o200)
}

func restoreRootEntryMode(root *os.Root, name string, mode fs.FileMode) error {
	return root.Chmod(name, mode.Perm())
}
