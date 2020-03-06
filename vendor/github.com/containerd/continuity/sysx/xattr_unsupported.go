// +build !linux,!darwin

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

package sysx

import (
	"errors"
	"runtime"
)

var unsupported = errors.New("extended attributes unsupported on " + runtime.GOOS)

// Listxattr calls syscall listxattr and reads all content
// and returns a string array
func Listxattr(path string) ([]string, error) {
	return []string{}, nil
}

// Removexattr calls syscall removexattr
func Removexattr(path string, attr string) (err error) {
	return unsupported
}

// Setxattr calls syscall setxattr
func Setxattr(path string, attr string, data []byte, flags int) (err error) {
	return unsupported
}

// Getxattr calls syscall getxattr
func Getxattr(path, attr string) ([]byte, error) {
	return []byte{}, unsupported
}

// LListxattr lists xattrs, not following symlinks
func LListxattr(path string) ([]string, error) {
	return []string{}, nil
}

// LRemovexattr removes an xattr, not following symlinks
func LRemovexattr(path string, attr string) (err error) {
	return unsupported
}

// LSetxattr sets an xattr, not following symlinks
func LSetxattr(path string, attr string, data []byte, flags int) (err error) {
	return unsupported
}

// LGetxattr gets an xattr, not following symlinks
func LGetxattr(path, attr string) ([]byte, error) {
	return []byte{}, nil
}
