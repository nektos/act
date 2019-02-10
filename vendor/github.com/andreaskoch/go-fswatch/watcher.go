// Copyright 2013 Andreas Koch. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fswatch

type Watcher interface {
	Modified() chan bool
	Moved() chan bool
	Stopped() chan bool

	Start()
	Stop()
	IsRunning() bool
}
