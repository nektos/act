// +build darwin freebsd

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
	"syscall"
	"time"
)

// StatAtime returns the access time from a stat struct
func StatAtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Atimespec
}

// StatCtime returns the created time from a stat struct
func StatCtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Ctimespec
}

// StatMtime returns the modified time from a stat struct
func StatMtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Mtimespec
}

// StatATimeAsTime returns the access time as a time.Time
func StatATimeAsTime(st *syscall.Stat_t) time.Time {
	return time.Unix(int64(st.Atimespec.Sec), int64(st.Atimespec.Nsec)) // nolint: unconvert
}
