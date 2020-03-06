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

import "context"

// Usage of disk information
type Usage struct {
	Inodes int64
	Size   int64
}

// DiskUsage counts the number of inodes and disk usage for the resources under
// path.
func DiskUsage(ctx context.Context, roots ...string) (Usage, error) {
	return diskUsage(ctx, roots...)
}

// DiffUsage counts the numbers of inodes and disk usage in the
// diff between the 2 directories. The first path is intended
// as the base directory and the second as the changed directory.
func DiffUsage(ctx context.Context, a, b string) (Usage, error) {
	return diffUsage(ctx, a, b)
}
