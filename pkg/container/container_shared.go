package container

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/pkg/errors"
)

type fileCollectorHandler interface {
	WriteFile(path string, fi fs.FileInfo, linkName string, f io.Reader) error
}

type tarCollector struct {
	TarWriter *tar.Writer
}

func (tc tarCollector) WriteFile(path string, fi fs.FileInfo, linkName string, f io.Reader) error {
	// create a new dir/file header
	header, err := tar.FileInfoHeader(fi, linkName)
	if err != nil {
		return err
	}

	// update the name to correctly reflect the desired destination when untaring
	header.Name = path
	header.Mode = int64(fi.Mode())
	header.ModTime = fi.ModTime()

	// write the header
	if err := tc.TarWriter.WriteHeader(header); err != nil {
		return err
	}

	// this is a symlink no reader provided
	if f == nil {
		return nil
	}

	// copy file data into tar writer
	if _, err := io.Copy(tc.TarWriter, f); err != nil {
		return err
	}
	return nil
}

type fileCollector struct {
	Ignorer   gitignore.Matcher
	SrcPath   string
	SrcPrefix string
	Context   context.Context
	Handler   fileCollectorHandler
}

func (fc *fileCollector) collectFiles(submodulePath []string) func(file string, fi os.FileInfo, err error) error {
	var i *index.Index
	if r, err := git.PlainOpen(path.Join(fc.SrcPath, path.Join(submodulePath...))); err == nil {
		i, err = r.Storer.Index()
		if err != nil {
			i = nil
		}
	}
	return func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fc.Context != nil {
			select {
			case <-fc.Context.Done():
				return fmt.Errorf("copy cancelled")
			default:
			}
		}

		sansPrefix := strings.TrimPrefix(file, fc.SrcPrefix)
		split := strings.Split(sansPrefix, string(filepath.Separator))
		// The root folders should be skipped, submodules only have the last path component set to "." by filepath.Walk
		if fi.IsDir() && len(split) > 0 && split[len(split)-1] == "." {
			return nil
		}
		var entry *index.Entry
		if i != nil {
			entry, err = i.Entry(strings.Join(split[len(submodulePath):], "/"))
		} else {
			err = index.ErrEntryNotFound
		}
		if err != nil && fc.Ignorer != nil && fc.Ignorer.Match(split, fi.IsDir()) {
			if fi.IsDir() {
				if i != nil {
					ms, err := i.Glob(strings.Join(append(split[len(submodulePath):], "**"), "/"))
					if err != nil || len(ms) == 0 {
						return filepath.SkipDir
					}
				} else {
					return filepath.SkipDir
				}
			} else {
				return nil
			}
		}
		if err == nil && entry.Mode == filemode.Submodule {
			err = filepath.Walk(fi.Name(), fc.collectFiles(split))
			if err != nil {
				return err
			}
			return filepath.SkipDir
		}
		path := filepath.ToSlash(sansPrefix)

		// return on non-regular files (thanks to [kumo](https://medium.com/@komuw/just-like-you-did-fbdd7df829d3) for this suggested update)
		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			linkName, err := os.Readlink(file)
			if err != nil {
				return errors.WithMessagef(err, "unable to readlink %s", file)
			}
			return fc.Handler.WriteFile(path, fi, linkName, nil)
		} else if !fi.Mode().IsRegular() {
			return nil
		}

		// open file
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		if fc.Context != nil {
			// make io.Copy cancellable by closing the file
			cpctx, cpfinish := context.WithCancel(fc.Context)
			defer cpfinish()
			go func() {
				select {
				case <-cpctx.Done():
				case <-fc.Context.Done():
					f.Close()
				}
			}()
		}

		return fc.Handler.WriteFile(path, fi, "", f)
	}
}
