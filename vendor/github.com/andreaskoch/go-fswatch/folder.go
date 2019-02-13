// Copyright 2013 Andreas Koch. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fswatch

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"
)

var numberOfFolderWatchers int

func init() {
	numberOfFolderWatchers = 0
}

func NumberOfFolderWatchers() int {
	return numberOfFolderWatchers
}

type FolderWatcher struct {
	changeDetails chan *FolderChange

	modified chan bool
	moved    chan bool
	stopped  chan bool

	recurse  bool
	skipFile func(path string) bool

	debug         bool
	folder        string
	running       bool
	wasStopped    bool
	checkInterval time.Duration

	previousEntries []string
}

func NewFolderWatcher(folderPath string, recurse bool, skipFile func(path string) bool, checkIntervalInSeconds int) *FolderWatcher {

	if checkIntervalInSeconds < 1 {
		panic(fmt.Sprintf("Cannot create a folder watcher with a check interval of %v seconds.", checkIntervalInSeconds))
	}

	return &FolderWatcher{

		modified: make(chan bool),
		moved:    make(chan bool),
		stopped:  make(chan bool),

		changeDetails: make(chan *FolderChange),

		recurse:  recurse,
		skipFile: skipFile,

		debug:         true,
		folder:        folderPath,
		checkInterval: time.Duration(checkIntervalInSeconds),
	}
}

func (folderWatcher *FolderWatcher) String() string {
	return fmt.Sprintf("Folderwatcher %q", folderWatcher.folder)
}

func (folderWatcher *FolderWatcher) Modified() chan bool {
	return folderWatcher.modified
}

func (folderWatcher *FolderWatcher) Moved() chan bool {
	return folderWatcher.moved
}

func (folderWatcher *FolderWatcher) Stopped() chan bool {
	return folderWatcher.stopped
}

func (folderWatcher *FolderWatcher) ChangeDetails() chan *FolderChange {
	return folderWatcher.changeDetails
}

func (folderWatcher *FolderWatcher) Start() {
	folderWatcher.running = true
	sleepInterval := time.Second * folderWatcher.checkInterval

	go func() {

		// get existing entries
		var entryList []string
		directory := folderWatcher.folder

		previousEntryList := folderWatcher.getPreviousEntryList()

		if previousEntryList != nil {

			// use the entry list from a previous run
			entryList = previousEntryList

		} else {

			// use a new entry list
			newEntryList, _ := getFolderEntries(directory, folderWatcher.recurse, folderWatcher.skipFile)
			entryList = newEntryList
		}

		// increment watcher count
		numberOfFolderWatchers++

		for folderWatcher.wasStopped == false {

			// get new entries
			updatedEntryList, _ := getFolderEntries(directory, folderWatcher.recurse, folderWatcher.skipFile)

			// check for new items
			newItems := make([]string, 0)
			modifiedItems := make([]string, 0)

			for _, entry := range updatedEntryList {

				if isNewItem := !sliceContainsElement(entryList, entry); isNewItem {
					// entry is new
					newItems = append(newItems, entry)
					continue
				}

				// check if the file changed
				if newModTime, err := getLastModTimeFromFile(entry); err == nil {

					// check if file has been modified
					timeOfLastCheck := time.Now().Add(sleepInterval * -1)

					if timeOfLastCheck.Before(newModTime) {

						// existing entry has been modified
						modifiedItems = append(modifiedItems, entry)
					}

				}
			}

			// check for moved items
			movedItems := make([]string, 0)
			for _, entry := range entryList {
				isMoved := !sliceContainsElement(updatedEntryList, entry)
				if isMoved {
					movedItems = append(movedItems, entry)
				}
			}

			// assign the new list
			entryList = updatedEntryList

			// sleep
			time.Sleep(sleepInterval)

			// check if something happened
			if len(newItems) > 0 || len(movedItems) > 0 || len(modifiedItems) > 0 {

				// send out change
				go func() {
					folderWatcher.modified <- true
				}()

				go func() {
					log("Folder %q changed", directory)
					folderWatcher.changeDetails <- newFolderChange(newItems, movedItems, modifiedItems)
				}()
			} else {
				log("No change in folder %q", directory)
			}
		}

		folderWatcher.running = false

		// capture the entry list for a restart
		folderWatcher.captureEntryList(entryList)

		// inform channel-subscribers
		go func() {
			folderWatcher.stopped <- true
		}()

		// decrement the watch counter
		numberOfFolderWatchers--

		// final log message
		log("Stopped folder watcher %q", folderWatcher.String())
	}()
}

func (folderWatcher *FolderWatcher) Stop() {
	log("Stopping folder watcher %q", folderWatcher.String())
	folderWatcher.wasStopped = true
}

func (folderWatcher *FolderWatcher) IsRunning() bool {
	return folderWatcher.running
}

func (folderWatcher *FolderWatcher) getPreviousEntryList() []string {
	return folderWatcher.previousEntries
}

// Remember the entry list for a later restart
func (folderWatcher *FolderWatcher) captureEntryList(list []string) {
	folderWatcher.previousEntries = list
}

func getFolderEntries(directory string, recurse bool, skipFile func(path string) bool) ([]string, error) {

	// the return array
	entries := make([]string, 0)

	// read the entries of the specified directory
	directoryEntries, err := ioutil.ReadDir(directory)
	if err != nil {
		return entries, err
	}

	for _, entry := range directoryEntries {

		// get the full path
		subEntryPath := filepath.Join(directory, entry.Name())

		// recurse or append
		if recurse && entry.IsDir() {

			// recurse (ignore errors, unreadable sub directories don't hurt much)
			subFolderEntries, _ := getFolderEntries(subEntryPath, recurse, skipFile)
			entries = append(entries, subFolderEntries...)

		} else {

			// check if the enty shall be ignored
			if skipFile(subEntryPath) {
				continue
			}

			// append entry
			entries = append(entries, subEntryPath)
		}

	}

	return entries, nil
}
