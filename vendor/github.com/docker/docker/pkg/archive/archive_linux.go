package archive // import "github.com/docker/docker/pkg/archive"

import (
	"archive/tar"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func getWhiteoutConverter(format WhiteoutFormat, inUserNS bool) tarWhiteoutConverter {
	if format == OverlayWhiteoutFormat {
		return overlayWhiteoutConverter{inUserNS: inUserNS}
	}
	return nil
}

type overlayWhiteoutConverter struct {
	inUserNS bool
}

func (overlayWhiteoutConverter) ConvertWrite(hdr *tar.Header, path string, fi os.FileInfo) (wo *tar.Header, err error) {
	// convert whiteouts to AUFS format
	if fi.Mode()&os.ModeCharDevice != 0 && hdr.Devmajor == 0 && hdr.Devminor == 0 {
		// we just rename the file and make it normal
		dir, filename := filepath.Split(hdr.Name)
		hdr.Name = filepath.Join(dir, WhiteoutPrefix+filename)
		hdr.Mode = 0600
		hdr.Typeflag = tar.TypeReg
		hdr.Size = 0
	}

	if fi.Mode()&os.ModeDir != 0 {
		// convert opaque dirs to AUFS format by writing an empty file with the prefix
		opaque, err := system.Lgetxattr(path, "trusted.overlay.opaque")
		if err != nil {
			return nil, err
		}
		if len(opaque) == 1 && opaque[0] == 'y' {
			if hdr.Xattrs != nil {
				delete(hdr.Xattrs, "trusted.overlay.opaque")
			}

			// create a header for the whiteout file
			// it should inherit some properties from the parent, but be a regular file
			wo = &tar.Header{
				Typeflag:   tar.TypeReg,
				Mode:       hdr.Mode & int64(os.ModePerm),
				Name:       filepath.Join(hdr.Name, WhiteoutOpaqueDir),
				Size:       0,
				Uid:        hdr.Uid,
				Uname:      hdr.Uname,
				Gid:        hdr.Gid,
				Gname:      hdr.Gname,
				AccessTime: hdr.AccessTime,
				ChangeTime: hdr.ChangeTime,
			}
		}
	}

	return
}

func (c overlayWhiteoutConverter) ConvertRead(hdr *tar.Header, path string) (bool, error) {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// if a directory is marked as opaque by the AUFS special file, we need to translate that to overlay
	if base == WhiteoutOpaqueDir {
		err := unix.Setxattr(dir, "trusted.overlay.opaque", []byte{'y'}, 0)
		if err != nil {
			if c.inUserNS {
				if err = replaceDirWithOverlayOpaque(dir); err != nil {
					return false, errors.Wrapf(err, "replaceDirWithOverlayOpaque(%q) failed", dir)
				}
			} else {
				return false, errors.Wrapf(err, "setxattr(%q, trusted.overlay.opaque=y)", dir)
			}
		}
		// don't write the file itself
		return false, err
	}

	// if a file was deleted and we are using overlay, we need to create a character device
	if strings.HasPrefix(base, WhiteoutPrefix) {
		originalBase := base[len(WhiteoutPrefix):]
		originalPath := filepath.Join(dir, originalBase)

		if err := unix.Mknod(originalPath, unix.S_IFCHR, 0); err != nil {
			if c.inUserNS {
				// Ubuntu and a few distros support overlayfs in userns.
				//
				// Although we can't call mknod directly in userns (at least on bionic kernel 4.15),
				// we can still create 0,0 char device using mknodChar0Overlay().
				//
				// NOTE: we don't need this hack for the containerd snapshotter+unpack model.
				if err := mknodChar0Overlay(originalPath); err != nil {
					return false, errors.Wrapf(err, "failed to mknodChar0UserNS(%q)", originalPath)
				}
			} else {
				return false, errors.Wrapf(err, "failed to mknod(%q, S_IFCHR, 0)", originalPath)
			}
		}
		if err := os.Chown(originalPath, hdr.Uid, hdr.Gid); err != nil {
			return false, err
		}

		// don't write the file itself
		return false, nil
	}

	return true, nil
}

// mknodChar0Overlay creates 0,0 char device by mounting overlayfs and unlinking.
// This function can be used for creating 0,0 char device in userns on Ubuntu.
//
// Steps:
// * Mkdir lower,upper,merged,work
// * Create lower/dummy
// * Mount overlayfs
// * Unlink merged/dummy
// * Unmount overlayfs
// * Make sure a 0,0 char device is created as upper/dummy
// * Rename upper/dummy to cleansedOriginalPath
func mknodChar0Overlay(cleansedOriginalPath string) error {
	dir := filepath.Dir(cleansedOriginalPath)
	tmp, err := ioutil.TempDir(dir, "mc0o")
	if err != nil {
		return errors.Wrapf(err, "failed to create a tmp directory under %s", dir)
	}
	defer os.RemoveAll(tmp)
	lower := filepath.Join(tmp, "l")
	upper := filepath.Join(tmp, "u")
	work := filepath.Join(tmp, "w")
	merged := filepath.Join(tmp, "m")
	for _, s := range []string{lower, upper, work, merged} {
		if err := os.MkdirAll(s, 0700); err != nil {
			return errors.Wrapf(err, "failed to mkdir %s", s)
		}
	}
	dummyBase := "d"
	lowerDummy := filepath.Join(lower, dummyBase)
	if err := ioutil.WriteFile(lowerDummy, []byte{}, 0600); err != nil {
		return errors.Wrapf(err, "failed to create a dummy lower file %s", lowerDummy)
	}
	mOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	// docker/pkg/mount.Mount() requires procfs to be mounted. So we use syscall.Mount() directly instead.
	if err := syscall.Mount("overlay", merged, "overlay", uintptr(0), mOpts); err != nil {
		return errors.Wrapf(err, "failed to mount overlay (%s) on %s", mOpts, merged)
	}
	mergedDummy := filepath.Join(merged, dummyBase)
	if err := os.Remove(mergedDummy); err != nil {
		syscall.Unmount(merged, 0)
		return errors.Wrapf(err, "failed to unlink %s", mergedDummy)
	}
	if err := syscall.Unmount(merged, 0); err != nil {
		return errors.Wrapf(err, "failed to unmount %s", merged)
	}
	upperDummy := filepath.Join(upper, dummyBase)
	if err := isChar0(upperDummy); err != nil {
		return err
	}
	if err := os.Rename(upperDummy, cleansedOriginalPath); err != nil {
		return errors.Wrapf(err, "failed to rename %s to %s", upperDummy, cleansedOriginalPath)
	}
	return nil
}

func isChar0(path string) error {
	osStat, err := os.Stat(path)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %s", path)
	}
	st, ok := osStat.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.Errorf("got unsupported stat for %s", path)
	}
	if os.FileMode(st.Mode)&syscall.S_IFMT != syscall.S_IFCHR {
		return errors.Errorf("%s is not a character device, got mode=%d", path, st.Mode)
	}
	if st.Rdev != 0 {
		return errors.Errorf("%s is not a 0,0 character device, got Rdev=%d", path, st.Rdev)
	}
	return nil
}

// replaceDirWithOverlayOpaque replaces path with a new directory with trusted.overlay.opaque
// xattr. The contents of the directory are preserved.
func replaceDirWithOverlayOpaque(path string) error {
	if path == "/" {
		return errors.New("replaceDirWithOverlayOpaque: path must not be \"/\"")
	}
	dir := filepath.Dir(path)
	tmp, err := ioutil.TempDir(dir, "rdwoo")
	if err != nil {
		return errors.Wrapf(err, "failed to create a tmp directory under %s", dir)
	}
	defer os.RemoveAll(tmp)
	// newPath is a new empty directory crafted with trusted.overlay.opaque xattr.
	// we copy the content of path into newPath, remove path, and rename newPath to path.
	newPath, err := createDirWithOverlayOpaque(tmp)
	if err != nil {
		return errors.Wrapf(err, "createDirWithOverlayOpaque(%q) failed", tmp)
	}
	if err := fs.CopyDir(newPath, path); err != nil {
		return errors.Wrapf(err, "CopyDir(%q, %q) failed", newPath, path)
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.Rename(newPath, path)
}

// createDirWithOverlayOpaque creates a directory with trusted.overlay.opaque xattr,
// without calling setxattr, so as to allow creating opaque dir in userns on Ubuntu.
func createDirWithOverlayOpaque(tmp string) (string, error) {
	lower := filepath.Join(tmp, "l")
	upper := filepath.Join(tmp, "u")
	work := filepath.Join(tmp, "w")
	merged := filepath.Join(tmp, "m")
	for _, s := range []string{lower, upper, work, merged} {
		if err := os.MkdirAll(s, 0700); err != nil {
			return "", errors.Wrapf(err, "failed to mkdir %s", s)
		}
	}
	dummyBase := "d"
	lowerDummy := filepath.Join(lower, dummyBase)
	if err := os.MkdirAll(lowerDummy, 0700); err != nil {
		return "", errors.Wrapf(err, "failed to create a dummy lower directory %s", lowerDummy)
	}
	mOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	// docker/pkg/mount.Mount() requires procfs to be mounted. So we use syscall.Mount() directly instead.
	if err := syscall.Mount("overlay", merged, "overlay", uintptr(0), mOpts); err != nil {
		return "", errors.Wrapf(err, "failed to mount overlay (%s) on %s", mOpts, merged)
	}
	mergedDummy := filepath.Join(merged, dummyBase)
	if err := os.Remove(mergedDummy); err != nil {
		syscall.Unmount(merged, 0)
		return "", errors.Wrapf(err, "failed to rmdir %s", mergedDummy)
	}
	// upperDummy becomes a 0,0-char device file here
	if err := os.Mkdir(mergedDummy, 0700); err != nil {
		syscall.Unmount(merged, 0)
		return "", errors.Wrapf(err, "failed to mkdir %s", mergedDummy)
	}
	// upperDummy becomes a directory with trusted.overlay.opaque xattr
	// (but can't be verified in userns)
	if err := syscall.Unmount(merged, 0); err != nil {
		return "", errors.Wrapf(err, "failed to unmount %s", merged)
	}
	upperDummy := filepath.Join(upper, dummyBase)
	return upperDummy, nil
}
