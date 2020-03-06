// +build !windows

package archive // import "github.com/docker/docker/pkg/archive"

import (
	"os"
	"syscall"

	"github.com/docker/docker/pkg/system"
	"golang.org/x/sys/unix"
)

func statDifferent(oldStat *system.StatT, newStat *system.StatT) bool {
	// Don't look at size for dirs, its not a good measure of change
	if oldStat.Mode() != newStat.Mode() ||
		oldStat.UID() != newStat.UID() ||
		oldStat.GID() != newStat.GID() ||
		oldStat.Rdev() != newStat.Rdev() ||
		// Don't look at size or modification time for dirs, its not a good
		// measure of change. See https://github.com/moby/moby/issues/9874
		// for a description of the issue with modification time, and
		// https://github.com/moby/moby/pull/11422 for the change.
		// (Note that in the Windows implementation of this function,
		// modification time IS taken as a change). See
		// https://github.com/moby/moby/pull/37982 for more information.
		(oldStat.Mode()&unix.S_IFDIR != unix.S_IFDIR &&
			(!sameFsTimeSpec(oldStat.Mtim(), newStat.Mtim()) || (oldStat.Size() != newStat.Size()))) {
		return true
	}
	return false
}

func (info *FileInfo) isDir() bool {
	return info.parent == nil || info.stat.Mode()&unix.S_IFDIR != 0
}

func getIno(fi os.FileInfo) uint64 {
	return fi.Sys().(*syscall.Stat_t).Ino
}

func hasHardlinks(fi os.FileInfo) bool {
	return fi.Sys().(*syscall.Stat_t).Nlink > 1
}
