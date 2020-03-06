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
	"os"

	"golang.org/x/sys/windows"
)

func detectDirDiff(upper, lower string) *diffDirOptions {
	return nil
}

func compareSysStat(s1, s2 interface{}) (bool, error) {
	f1, ok := s1.(windows.Win32FileAttributeData)
	if !ok {
		return false, nil
	}
	f2, ok := s2.(windows.Win32FileAttributeData)
	if !ok {
		return false, nil
	}
	return f1.FileAttributes == f2.FileAttributes, nil
}

func compareCapabilities(p1, p2 string) (bool, error) {
	// TODO: Use windows equivalent
	return true, nil
}

func isLinked(os.FileInfo) bool {
	return false
}
