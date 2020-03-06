/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package fs

import (
	"io"
	"os"
	"syscall"

	"github.com/containerd/continuity/sysx"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func copyFileInfo(fi os.FileInfo, name string) error {
	st := fi.Sys().(*syscall.Stat_t)
	if err := os.Lchown(name, int(st.Uid), int(st.Gid)); err != nil {
		if os.IsPermission(err) {
			// Normally if uid/gid are the same this would be a no-op, but some
			// filesystems may still return EPERM... for instance NFS does this.
			// In such a case, this is not an error.
			if dstStat, err2 := os.Lstat(name); err2 == nil {
				st2 := dstStat.Sys().(*syscall.Stat_t)
				if st.Uid == st2.Uid && st.Gid == st2.Gid {
					err = nil
				}
			}
		}
		if err != nil {
			return errors.Wrapf(err, "failed to chown %s", name)
		}
	}

	if (fi.Mode() & os.ModeSymlink) != os.ModeSymlink {
		if err := os.Chmod(name, fi.Mode()); err != nil {
			return errors.Wrapf(err, "failed to chmod %s", name)
		}
	}

	timespec := []unix.Timespec{unix.Timespec(StatAtime(st)), unix.Timespec(StatMtime(st))}
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, name, timespec, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return errors.Wrapf(err, "failed to utime %s", name)
	}

	return nil
}

const maxSSizeT = int64(^uint(0) >> 1)

func copyFileContent(dst, src *os.File) error {
	st, err := src.Stat()
	if err != nil {
		return errors.Wrap(err, "unable to stat source")
	}

	size := st.Size()
	first := true
	srcFd := int(src.Fd())
	dstFd := int(dst.Fd())

	for size > 0 {
		// Ensure that we are never trying to copy more than SSIZE_MAX at a
		// time and at the same time avoids overflows when the file is larger
		// than 4GB on 32-bit systems.
		var copySize int
		if size > maxSSizeT {
			copySize = int(maxSSizeT)
		} else {
			copySize = int(size)
		}
		n, err := unix.CopyFileRange(srcFd, nil, dstFd, nil, copySize, 0)
		if err != nil {
			if (err != unix.ENOSYS && err != unix.EXDEV) || !first {
				return errors.Wrap(err, "copy file range failed")
			}

			buf := bufferPool.Get().(*[]byte)
			_, err = io.CopyBuffer(dst, src, *buf)
			bufferPool.Put(buf)
			return errors.Wrap(err, "userspace copy failed")
		}

		first = false
		size -= int64(n)
	}

	return nil
}

func copyXAttrs(dst, src string, xeh XAttrErrorHandler) error {
	xattrKeys, err := sysx.LListxattr(src)
	if err != nil {
		e := errors.Wrapf(err, "failed to list xattrs on %s", src)
		if xeh != nil {
			e = xeh(dst, src, "", e)
		}
		return e
	}
	for _, xattr := range xattrKeys {
		data, err := sysx.LGetxattr(src, xattr)
		if err != nil {
			e := errors.Wrapf(err, "failed to get xattr %q on %s", xattr, src)
			if xeh != nil {
				if e = xeh(dst, src, xattr, e); e == nil {
					continue
				}
			}
			return e
		}
		if err := sysx.LSetxattr(dst, xattr, data, 0); err != nil {
			e := errors.Wrapf(err, "failed to set xattr %q on %s", xattr, dst)
			if xeh != nil {
				if e = xeh(dst, src, xattr, e); e == nil {
					continue
				}
			}
			return e
		}
	}

	return nil
}

func copyDevice(dst string, fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("unsupported stat type")
	}
	return unix.Mknod(dst, uint32(fi.Mode()), int(st.Rdev))
}
