// Copyright 2013 Andreas Koch. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fswatch

import (
	"fmt"
	"time"
)

type FolderChange struct {
	timeStamp     time.Time
	newItems      []string
	movedItems    []string
	modifiedItems []string
}

func newFolderChange(newItems, movedItems, modifiedItems []string) *FolderChange {
	return &FolderChange{
		timeStamp:     time.Now(),
		newItems:      newItems,
		movedItems:    movedItems,
		modifiedItems: modifiedItems,
	}
}

func (folderChange *FolderChange) String() string {
	return fmt.Sprintf("Folderchange (timestamp: %s, new: %d, moved: %d)", folderChange.timeStamp, len(folderChange.New()), len(folderChange.Moved()))
}

func (folderChange *FolderChange) TimeStamp() time.Time {
	return folderChange.timeStamp
}

func (folderChange *FolderChange) New() []string {
	return folderChange.newItems
}

func (folderChange *FolderChange) Moved() []string {
	return folderChange.movedItems
}

func (folderChange *FolderChange) Modified() []string {
	return folderChange.modifiedItems
}
