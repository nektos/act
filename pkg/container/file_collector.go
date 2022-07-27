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
)

type fileCollectorHandler interface {
	WriteFile(path string, fi fs.FileInfo, linkName string, f io.Reader) error
}

type tarCollector struct {
	TarWriter *tar.Writer
	UID       int
	GID       int
	DstDir    string
}

func (tc tarCollector) WriteFile(fpath string, fi fs.FileInfo, linkName string, f io.Reader) error {
	// create a new dir/file header
	header, err := tar.FileInfoHeader(fi, linkName)
	if err != nil {
		return err
	}

	// update the name to correctly reflect the desired destination when untaring
	header.Name = path.Join(tc.DstDir, fpath)
	header.Mode = int64(fi.Mode())
	header.ModTime = fi.ModTime()
	header.Uid = tc.UID
	header.Gid = tc.GID

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
	Fs        fileCollectorFs
	Handler   fileCollectorHandler
}

type fileCollectorFs interface {
	Walk(root string, fn filepath.WalkFunc) error
	OpenGitIndex(path string) (*index.Index, error)
	Open(path string) (io.ReadCloser, error)
	Readlink(path string) (string, error)
}

type defaultFs struct {
}

func (*defaultFs) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

func (*defaultFs) OpenGitIndex(path string) (*index.Index, error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	i, err := r.Storer.Index()
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (*defaultFs) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (*defaultFs) Readlink(path string) (string, error) {
	return os.Readlink(path)
}

// nolint: gocyclo
func (fc *fileCollector) collectFiles(ctx context.Context, submodulePath []string) filepath.WalkFunc {
	i, _ := fc.Fs.OpenGitIndex(path.Join(fc.SrcPath, path.Join(submodulePath...)))
	return func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if ctx != nil {
			select {
			case <-ctx.Done():
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

		path := filepath.ToSlash(sansPrefix)

		if err == nil && entry.Mode == filemode.Submodule {
			err = fc.Fs.Walk(path, fc.collectFiles(ctx, split))
			if err != nil {
				return err
			}
			return filepath.SkipDir
		}

		// return on non-regular files (thanks to [kumo](https://medium.com/@komuw/just-like-you-did-fbdd7df829d3) for this suggested update)
		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			linkName, err := fc.Fs.Readlink(file)
			if err != nil {
				return fmt.Errorf("unable to readlink '%s': %w", file, err)
			}
			return fc.Handler.WriteFile(path, fi, linkName, nil)
		} else if !fi.Mode().IsRegular() {
			return nil
		}

		// open file
		f, err := fc.Fs.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		if ctx != nil {
			// make io.Copy cancellable by closing the file
			cpctx, cpfinish := context.WithCancel(ctx)
			defer cpfinish()
			go func() {
				select {
				case <-cpctx.Done():
				case <-ctx.Done():
					f.Close()
				}
			}()
		}

		return fc.Handler.WriteFile(path, fi, "", f)
	}
}
