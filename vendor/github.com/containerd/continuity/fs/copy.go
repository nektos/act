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
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
)

var bufferPool = &sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 32*1024)
		return &buffer
	},
}

// XAttrErrorHandlers transform a non-nil xattr error.
// Return nil to ignore an error.
// xattrKey can be empty for listxattr operation.
type XAttrErrorHandler func(dst, src, xattrKey string, err error) error

type copyDirOpts struct {
	xeh XAttrErrorHandler
}

type CopyDirOpt func(*copyDirOpts) error

// WithXAttrErrorHandler allows specifying XAttrErrorHandler
// If nil XAttrErrorHandler is specified (default), CopyDir stops
// on a non-nil xattr error.
func WithXAttrErrorHandler(xeh XAttrErrorHandler) CopyDirOpt {
	return func(o *copyDirOpts) error {
		o.xeh = xeh
		return nil
	}
}

// WithAllowXAttrErrors allows ignoring xattr errors.
func WithAllowXAttrErrors() CopyDirOpt {
	xeh := func(dst, src, xattrKey string, err error) error {
		return nil
	}
	return WithXAttrErrorHandler(xeh)
}

// CopyDir copies the directory from src to dst.
// Most efficient copy of files is attempted.
func CopyDir(dst, src string, opts ...CopyDirOpt) error {
	var o copyDirOpts
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return err
		}
	}
	inodes := map[uint64]string{}
	return copyDirectory(dst, src, inodes, &o)
}

func copyDirectory(dst, src string, inodes map[uint64]string, o *copyDirOpts) error {
	stat, err := os.Stat(src)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %s", src)
	}
	if !stat.IsDir() {
		return errors.Errorf("source is not directory")
	}

	if st, err := os.Stat(dst); err != nil {
		if err := os.Mkdir(dst, stat.Mode()); err != nil {
			return errors.Wrapf(err, "failed to mkdir %s", dst)
		}
	} else if !st.IsDir() {
		return errors.Errorf("cannot copy to non-directory: %s", dst)
	} else {
		if err := os.Chmod(dst, stat.Mode()); err != nil {
			return errors.Wrapf(err, "failed to chmod on %s", dst)
		}
	}

	fis, err := ioutil.ReadDir(src)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", src)
	}

	if err := copyFileInfo(stat, dst); err != nil {
		return errors.Wrapf(err, "failed to copy file info for %s", dst)
	}

	for _, fi := range fis {
		source := filepath.Join(src, fi.Name())
		target := filepath.Join(dst, fi.Name())

		switch {
		case fi.IsDir():
			if err := copyDirectory(target, source, inodes, o); err != nil {
				return err
			}
			continue
		case (fi.Mode() & os.ModeType) == 0:
			link, err := getLinkSource(target, fi, inodes)
			if err != nil {
				return errors.Wrap(err, "failed to get hardlink")
			}
			if link != "" {
				if err := os.Link(link, target); err != nil {
					return errors.Wrap(err, "failed to create hard link")
				}
			} else if err := CopyFile(target, source); err != nil {
				return errors.Wrap(err, "failed to copy files")
			}
		case (fi.Mode() & os.ModeSymlink) == os.ModeSymlink:
			link, err := os.Readlink(source)
			if err != nil {
				return errors.Wrapf(err, "failed to read link: %s", source)
			}
			if err := os.Symlink(link, target); err != nil {
				return errors.Wrapf(err, "failed to create symlink: %s", target)
			}
		case (fi.Mode() & os.ModeDevice) == os.ModeDevice:
			if err := copyDevice(target, fi); err != nil {
				return errors.Wrapf(err, "failed to create device")
			}
		default:
			// TODO: Support pipes and sockets
			return errors.Wrapf(err, "unsupported mode %s", fi.Mode())
		}
		if err := copyFileInfo(fi, target); err != nil {
			return errors.Wrap(err, "failed to copy file info")
		}

		if err := copyXAttrs(target, source, o.xeh); err != nil {
			return errors.Wrap(err, "failed to copy xattrs")
		}
	}

	return nil
}

// CopyFile copies the source file to the target.
// The most efficient means of copying is used for the platform.
func CopyFile(target, source string) error {
	src, err := os.Open(source)
	if err != nil {
		return errors.Wrapf(err, "failed to open source %s", source)
	}
	defer src.Close()
	tgt, err := os.Create(target)
	if err != nil {
		return errors.Wrapf(err, "failed to open target %s", target)
	}
	defer tgt.Close()

	return copyFileContent(tgt, src)
}
