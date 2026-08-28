//go:build !windows

package filecollector

import (
	"io/fs"
	"os"
)

func prepareRootBackupRemoval(_ *os.Root, _ string, _ fs.FileMode) error {
	return nil
}

func restoreRootEntryMode(_ *os.Root, _ string, _ fs.FileMode) error {
	return nil
}
