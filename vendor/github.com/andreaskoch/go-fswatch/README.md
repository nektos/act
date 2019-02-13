# fswatch

fswatch is a go library for watching file system changes to **does not** depend on inotify.

## Motivation

Why not use [inotify](http://en.wikipedia.org/wiki/Inotify)? Even though there are great libraries like [fsnotify](https://github.com/howeyc/fsnotify) that offer cross platform file system change notifications - the approach breaks when you want to watch a lot of files or folder.

For example the default ulimit for Mac OS is set to 512. If you need to watch more files you have to increase the ulimit for open files per process. And this sucks.

## Usage

### Watching a single file

If you want to watch a single file use the `NewFileWatcher` function to create a new file watcher:

```go
go func() {
	fileWatcher := fswatch.NewFileWatcher("Some-file").Start()

	for fileWatcher.IsRunning() {

		select {
		case <-fileWatcher.Modified:

			go func() {
				// file changed. do something.
			}()

		case <-fileWatcher.Moved:

			go func() {
				// file moved. do something.
			}()
		}

	}
}()
```

### Watching a folder

To watch a whole folder for new, modified or deleted files you can use the `NewFolderWatcher` function.

Parameters:

1. The directory path
2. A flag indicating whether the folder shall be watched recursively
3. An expression which decides which files are skipped


```go
go func() {

	recurse := true

	skipNoFile := func(path string) bool {
		return false
	}	

	folderWatcher := fswatch.NewFolderWatcher("some-directory", recurse, skipNoFile).Start()

	for folderWatcher.IsRunning() {

		select {
		case <-folderWatcher.Change:

			go func() {
				// some file changed, was added, moved or deleted.
			}()

		}
	}

}()
```

## Build Status

[![Build Status](https://travis-ci.org/andreaskoch/go-fswatch.png?branch=master)](https://travis-ci.org/andreaskoch/go-fswatch)

## Contribute

If you have an idea

- how to reliably increase the limit for the maximum number of open files from within the application
- how to overcome the limitations of inotify without having to resort to checking the files for changes over and over again
- or how to make the existing code more efficient

please send me a message or a pull request. All contributions are welcome.