// Copyright 2013 Andreas Koch. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fswatch

import (
	"fmt"
)

var (
	debugIsEnabled = false
	debugMessages  chan string
)

func EnableDebug() chan string {
	debugIsEnabled = true
	debugMessages = make(chan string, 10)
	return debugMessages
}

func DisableDebug() {
	debugIsEnabled = false
	close(debugMessages)
}

func log(format string, v ...interface{}) {
	if !debugIsEnabled {
		return
	}

	debugMessages <- fmt.Sprint(fmt.Sprintf(format, v...))
}
