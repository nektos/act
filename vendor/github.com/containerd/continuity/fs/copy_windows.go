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

	"github.com/pkg/errors"
)

func copyFileInfo(fi os.FileInfo, name string) error {
	if err := os.Chmod(name, fi.Mode()); err != nil {
		return errors.Wrapf(err, "failed to chmod %s", name)
	}

	// TODO: copy windows specific metadata

	return nil
}

func copyFileContent(dst, src *os.File) error {
	buf := bufferPool.Get().(*[]byte)
	_, err := io.CopyBuffer(dst, src, *buf)
	bufferPool.Put(buf)
	return err
}

func copyXAttrs(dst, src string, xeh XAttrErrorHandler) error {
	return nil
}

func copyDevice(dst string, fi os.FileInfo) error {
	return errors.New("device copy not supported")
}
