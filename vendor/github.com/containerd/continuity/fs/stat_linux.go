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

// StatAtime returns the Atim
func StatAtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Atim
}

// StatCtime returns the Ctim
func StatCtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Ctim
}

// StatMtime returns the Mtim
func StatMtime(st *syscall.Stat_t) syscall.Timespec {
	return st.Mtim
}

// StatATimeAsTime returns st.Atim as a time.Time
func StatATimeAsTime(st *syscall.Stat_t) time.Time {
	// The int64 conversions ensure the line compiles for 32-bit systems as well.
	return time.Unix(int64(st.Atim.Sec), int64(st.Atim.Nsec)) // nolint: unconvert
}
