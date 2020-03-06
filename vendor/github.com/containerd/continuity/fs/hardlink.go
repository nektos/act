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

import "os"

// GetLinkInfo returns an identifier representing the node a hardlink is pointing
// to. If the file is not hard linked then 0 will be returned.
func GetLinkInfo(fi os.FileInfo) (uint64, bool) {
	return getLinkInfo(fi)
}

// getLinkSource returns a path for the given name and
// file info to its link source in the provided inode
// map. If the given file name is not in the map and
// has other links, it is added to the inode map
// to be a source for other link locations.
func getLinkSource(name string, fi os.FileInfo, inodes map[uint64]string) (string, error) {
	inode, isHardlink := getLinkInfo(fi)
	if !isHardlink {
		return "", nil
	}

	path, ok := inodes[inode]
	if !ok {
		inodes[inode] = name
	}
	return path, nil
}
