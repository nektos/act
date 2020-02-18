// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package filepath implements utility routines for manipulating filename paths
// in a way compatible with the target operating system-defined file paths.
package walk

import (
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
)

// SkipDir is used as a return value from WalkFuncs to indicate that
// the directory named in the call is to be skipped. It is not returned
// as an error by any function.
var SkipDir = errors.New("skip this directory")

// WalkFunc is the type of the function called for each file or directory
// visited by Walk. The path argument contains the argument to Walk as a
// prefix; that is, if Walk is called with "dir", which is a directory
// containing the file "a", the walk function will be called with argument
// "dir/a". The info argument is the os.FileInfo for the named path.
//
// If there was a problem walking to the file or directory named by path, the
// incoming error will describe the problem and the function can decide how
// to handle that error (and Walk will not descend into that directory). If
// an error is returned, processing stops. The sole exception is that if path
// is a directory and the function returns the special value SkipDir, the
// contents of the directory are skipped and processing continues as usual on
// the next file.
type WalkFunc func(path string, info os.FileInfo, err error) error

var lstat = os.Lstat // for testing
var LstatP = &lstat

type VisitData struct {
	path string
	info os.FileInfo
}

type WalkState struct {
	walkFn     WalkFunc
	v          chan VisitData // files to be processed
	active     sync.WaitGroup // number of files to process
	lock       sync.RWMutex
	firstError error // accessed using lock
}

func (ws *WalkState) terminated() bool {
	ws.lock.RLock()
	done := ws.firstError != nil
	ws.lock.RUnlock()
	return done
}

func (ws *WalkState) setTerminated(err error) {
	ws.lock.Lock()
	if ws.firstError == nil {
		ws.firstError = err
	}
	ws.lock.Unlock()
	return
}

func (ws *WalkState) visitChannel() {
	for file := range ws.v {
		ws.visitFile(file)
		ws.active.Add(-1)
	}
}

func (ws *WalkState) visitFile(file VisitData) {
	if ws.terminated() {
		return
	}

	err := ws.walkFn(file.path, file.info, nil)
	if err != nil {
		if !(file.info.IsDir() && err == SkipDir) {
			ws.setTerminated(err)
		}
		return
	}

	if !file.info.IsDir() {
		return
	}

	names, err := readDirNames(file.path)
	if err != nil {
		err = ws.walkFn(file.path, file.info, err)
		if err != nil {
			ws.setTerminated(err)
		}
		return
	}

	here := file.path
	for _, name := range names {
		file.path = Join(here, name)
		file.info, err = lstat(file.path)
		if err != nil {
			err = ws.walkFn(file.path, file.info, err)
			if err != nil && (!file.info.IsDir() || err != SkipDir) {
				ws.setTerminated(err)
				return
			}
		} else {
			switch file.info.IsDir() {
			case true:
				ws.active.Add(1) // presume channel send will succeed
				select {
				case ws.v <- file:
					// push directory info to queue for concurrent traversal
				default:
					// undo increment when send fails and handle now
					ws.active.Add(-1)
					ws.visitFile(file)
				}
			case false:
				err = ws.walkFn(file.path, file.info, nil)
				if err != nil {
					ws.setTerminated(err)
					return
				}
			}
		}
	}
}

// Walk walks the file tree rooted at root, calling walkFn for each file or
// directory in the tree, including root. All errors that arise visiting files
// and directories are filtered by walkFn. The files are walked in a random
// order. Walk does not follow symbolic links.

func Walk(root string, walkFn WalkFunc) error {
	info, err := os.Lstat(root)
	if err != nil {
		return walkFn(root, nil, err)
	}

	ws := &WalkState{
		walkFn: walkFn,
		v:      make(chan VisitData, 1024),
	}
	defer close(ws.v)

	ws.active.Add(1)
	ws.v <- VisitData{root, info}

	walkers := 16
	for i := 0; i < walkers; i++ {
		go ws.visitChannel()
	}
	ws.active.Wait()

	return ws.firstError
}

//
// THE REMAINDER IS UNCHANGED FROM THE ORGINAL GO LIBRARY ORIGINAL
//

// readDirNames reads the directory named by dirname and returns
// a sorted list of directory entries.
func readDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	sort.Strings(names) // omit sort to save 1-2%
	return names, nil
}

// A lazybuf is a lazily constructed path buffer.
// It supports append, reading previously appended bytes,
// and retrieving the final string. It does not allocate a buffer
// to hold the output until that output diverges from s.
type lazybuf struct {
	path       string
	buf        []byte
	w          int
	volAndPath string
	volLen     int
}

func (b *lazybuf) index(i int) byte {
	if b.buf != nil {
		return b.buf[i]
	}
	return b.path[i]
}

func (b *lazybuf) append(c byte) {
	if b.buf == nil {
		if b.w < len(b.path) && b.path[b.w] == c {
			b.w++
			return
		}
		b.buf = make([]byte, len(b.path))
		copy(b.buf, b.path[:b.w])
	}
	b.buf[b.w] = c
	b.w++
}

func (b *lazybuf) string() string {
	if b.buf == nil {
		return b.volAndPath[:b.volLen+b.w]
	}
	return b.volAndPath[:b.volLen] + string(b.buf[:b.w])
}

const (
	Separator     = os.PathSeparator
	ListSeparator = os.PathListSeparator
)

// Clean returns the shortest path name equivalent to path
// by purely lexical processing.  It applies the following rules
// iteratively until no further processing can be done:
//
//	1. Replace multiple Separator elements with a single one.
//	2. Eliminate each . path name element (the current directory).
//	3. Eliminate each inner .. path name element (the parent directory)
//	   along with the non-.. element that precedes it.
//	4. Eliminate .. elements that begin a rooted path:
//	   that is, replace "/.." by "/" at the beginning of a path,
//	   assuming Separator is '/'.
//
// The returned path ends in a slash only if it represents a root directory,
// such as "/" on Unix or `C:\` on Windows.
//
// If the result of this process is an empty string, Clean
// returns the string ".".
//
// See also Rob Pike, ``Lexical File Names in Plan 9 or
// Getting Dot-Dot Right,''
// http://plan9.bell-labs.com/sys/doc/lexnames.html
func Clean(path string) string {
	originalPath := path
	volLen := volumeNameLen(path)
	path = path[volLen:]
	if path == "" {
		if volLen > 1 && originalPath[1] != ':' {
			// should be UNC
			return FromSlash(originalPath)
		}
		return originalPath + "."
	}
	rooted := os.IsPathSeparator(path[0])

	// Invariants:
	//	reading from path; r is index of next byte to process.
	//	writing to buf; w is index of next byte to write.
	//	dotdot is index in buf where .. must stop, either because
	//		it is the leading slash or it is a leading ../../.. prefix.
	n := len(path)
	out := lazybuf{path: path, volAndPath: originalPath, volLen: volLen}
	r, dotdot := 0, 0
	if rooted {
		out.append(Separator)
		r, dotdot = 1, 1
	}

	for r < n {
		switch {
		case os.IsPathSeparator(path[r]):
			// empty path element
			r++
		case path[r] == '.' && (r+1 == n || os.IsPathSeparator(path[r+1])):
			// . element
			r++
		case path[r] == '.' && path[r+1] == '.' && (r+2 == n || os.IsPathSeparator(path[r+2])):
			// .. element: remove to last separator
			r += 2
			switch {
			case out.w > dotdot:
				// can backtrack
				out.w--
				for out.w > dotdot && !os.IsPathSeparator(out.index(out.w)) {
					out.w--
				}
			case !rooted:
				// cannot backtrack, but not rooted, so append .. element.
				if out.w > 0 {
					out.append(Separator)
				}
				out.append('.')
				out.append('.')
				dotdot = out.w
			}
		default:
			// real path element.
			// add slash if needed
			if rooted && out.w != 1 || !rooted && out.w != 0 {
				out.append(Separator)
			}
			// copy element
			for ; r < n && !os.IsPathSeparator(path[r]); r++ {
				out.append(path[r])
			}
		}
	}

	// Turn empty string into "."
	if out.w == 0 {
		out.append('.')
	}

	return FromSlash(out.string())
}

// ToSlash returns the result of replacing each separator character
// in path with a slash ('/') character. Multiple separators are
// replaced by multiple slashes.
func ToSlash(path string) string {
	if Separator == '/' {
		return path
	}
	return strings.Replace(path, string(Separator), "/", -1)
}

// FromSlash returns the result of replacing each slash ('/') character
// in path with a separator character. Multiple slashes are replaced
// by multiple separators.
func FromSlash(path string) string {
	if Separator == '/' {
		return path
	}
	return strings.Replace(path, "/", string(Separator), -1)
}

// Join joins any number of path elements into a single path, adding
// a Separator if necessary. The result is Cleaned, in particular
// all empty strings are ignored.
func Join(elem ...string) string {
	for i, e := range elem {
		if e != "" {
			return Clean(strings.Join(elem[i:], string(Separator)))
		}
	}
	return ""
}

// Rel returns a relative path that is lexically equivalent to targpath when
// joined to basepath with an intervening separator. That is,
// Join(basepath, Rel(basepath, targpath)) is equivalent to targpath itself.
// On success, the returned path will always be relative to basepath,
// even if basepath and targpath share no elements.
// An error is returned if targpath can't be made relative to basepath or if
// knowing the current working directory would be necessary to compute it.
func Rel(basepath, targpath string) (string, error) {
	baseVol := VolumeName(basepath)
	targVol := VolumeName(targpath)
	base := Clean(basepath)
	targ := Clean(targpath)
	if targ == base {
		return ".", nil
	}
	base = base[len(baseVol):]
	targ = targ[len(targVol):]
	if base == "." {
		base = ""
	}
	// Can't use IsAbs - `\a` and `a` are both relative in Windows.
	baseSlashed := len(base) > 0 && base[0] == Separator
	targSlashed := len(targ) > 0 && targ[0] == Separator
	if baseSlashed != targSlashed || baseVol != targVol {
		return "", errors.New("Rel: can't make " + targ + " relative to " + base)
	}
	// Position base[b0:bi] and targ[t0:ti] at the first differing elements.
	bl := len(base)
	tl := len(targ)
	var b0, bi, t0, ti int
	for {
		for bi < bl && base[bi] != Separator {
			bi++
		}
		for ti < tl && targ[ti] != Separator {
			ti++
		}
		if targ[t0:ti] != base[b0:bi] {
			break
		}
		if bi < bl {
			bi++
		}
		if ti < tl {
			ti++
		}
		b0 = bi
		t0 = ti
	}
	if base[b0:bi] == ".." {
		return "", errors.New("Rel: can't make " + targ + " relative to " + base)
	}
	if b0 != bl {
		// Base elements left. Must go up before going down.
		seps := strings.Count(base[b0:bl], string(Separator))
		size := 2 + seps*3
		if tl != t0 {
			size += 1 + tl - t0
		}
		buf := make([]byte, size)
		n := copy(buf, "..")
		for i := 0; i < seps; i++ {
			buf[n] = Separator
			copy(buf[n+1:], "..")
			n += 3
		}
		if t0 != tl {
			buf[n] = Separator
			copy(buf[n+1:], targ[t0:])
		}
		return string(buf), nil
	}
	return targ[t0:], nil
}

// VolumeName returns leading volume name.
// Given "C:\foo\bar" it returns "C:" under windows.
// Given "\\host\share\foo" it returns "\\host\share".
// On other platforms it returns "".
func VolumeName(path string) (v string) {
	return path[:volumeNameLen(path)]
}

// EvalSymlinks returns the path name after the evaluation of any symbolic
// links.
// If path is relative the result will be relative to the current directory,
// unless one of the components is an absolute symbolic link.
func EvalSymlinks(path string) (string, error) {
	return evalSymlinks(path)
}
