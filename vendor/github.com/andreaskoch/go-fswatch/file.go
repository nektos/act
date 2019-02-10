// Copyright 2013 Andreas Koch. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fswatch

import (
	"fmt"
	"os"
	"time"
)

var numberOfFileWatchers int

func init() {
	numberOfFolderWatchers = 0
}

func NumberOfFileWatchers() int {
	return numberOfFileWatchers
}

type FileWatcher struct {
	modified chan bool
	moved    chan bool
	stopped  chan bool

	file          string
	running       bool
	wasStopped    bool
	checkInterval time.Duration

	previousModTime time.Time
}

func NewFileWatcher(filePath string, checkIntervalInSeconds int) *FileWatcher {

	if checkIntervalInSeconds < 1 {
		panic(fmt.Sprintf("Cannot create a file watcher with a check interval of %v seconds.", checkIntervalInSeconds))
	}

	return &FileWatcher{
		modified: make(chan bool),
		moved:    make(chan bool),
		stopped:  make(chan bool),

		file:          filePath,
		checkInterval: time.Duration(checkIntervalInSeconds),
	}
}

func (fileWatcher *FileWatcher) String() string {
	return fmt.Sprintf("Filewatcher %q", fileWatcher.file)
}

func (fileWatcher *FileWatcher) SetFile(filePath string) {
	fileWatcher.file = filePath
}

func (filewatcher *FileWatcher) Modified() chan bool {
	return filewatcher.modified
}

func (filewatcher *FileWatcher) Moved() chan bool {
	return filewatcher.moved
}

func (filewatcher *FileWatcher) Stopped() chan bool {
	return filewatcher.stopped
}

func (fileWatcher *FileWatcher) Start() {
	fileWatcher.running = true
	sleepInterval := time.Second * fileWatcher.checkInterval

	go func() {

		// increment watcher count
		numberOfFileWatchers++

		var modTime time.Time
		previousModTime := fileWatcher.getPreviousModTime()

		if timeIsSet(previousModTime) {
			modTime = previousModTime
		} else {
			currentModTime, err := getLastModTimeFromFile(fileWatcher.file)
			if err != nil {

				// send out the notification
				log("File %q has been moved or is inaccessible.", fileWatcher.file)
				go func() {
					fileWatcher.moved <- true
				}()

				// stop this file watcher
				fileWatcher.Stop()

			} else {

				modTime = currentModTime
			}

		}

		for fileWatcher.wasStopped == false {

			newModTime, err := getLastModTimeFromFile(fileWatcher.file)
			if err != nil {

				// send out the notification
				log("File %q has been moved.", fileWatcher.file)
				go func() {
					fileWatcher.moved <- true
				}()

				// stop this file watcher
				fileWatcher.Stop()

				continue
			}

			// detect changes
			if modTime.Before(newModTime) {

				// send out the notification
				log("File %q has been modified.", fileWatcher.file)
				go func() {
					fileWatcher.modified <- true
				}()

			} else {

				log("File %q has not changed.", fileWatcher.file)

			}

			// assign the new modtime
			modTime = newModTime

			time.Sleep(sleepInterval)

		}

		fileWatcher.running = false

		// capture the entry list for a restart
		fileWatcher.captureModTime(modTime)

		// inform channel-subscribers
		go func() {
			fileWatcher.stopped <- true
		}()

		// decrement the watch counter
		numberOfFileWatchers--

		// final log message
		log("Stopped file watcher %q", fileWatcher.String())
	}()
}

func (fileWatcher *FileWatcher) Stop() {
	log("Stopping file watcher %q", fileWatcher.String())
	fileWatcher.wasStopped = true
}

func (fileWatcher *FileWatcher) IsRunning() bool {
	return fileWatcher.running
}

func (fileWatcher *FileWatcher) getPreviousModTime() time.Time {
	return fileWatcher.previousModTime
}

// Remember the last mod time for a later restart
func (fileWatcher *FileWatcher) captureModTime(modTime time.Time) {
	fileWatcher.previousModTime = modTime
}

func getLastModTimeFromFile(file string) (time.Time, error) {
	fileInfo, err := os.Stat(file)
	if err != nil {
		return time.Time{}, err
	}

	return fileInfo.ModTime(), nil
}

func timeIsSet(t time.Time) bool {
	return time.Time{} == t
}
