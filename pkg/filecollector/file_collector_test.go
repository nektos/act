package filecollector

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/assert"
)

type memoryFs struct {
	billy.Filesystem
}

func (mfs *memoryFs) walk(root string, fn filepath.WalkFunc) error {
	dir, err := mfs.ReadDir(root)
	if err != nil {
		return err
	}
	for i := 0; i < len(dir); i++ {
		filename := filepath.Join(root, dir[i].Name())
		err = fn(filename, dir[i], nil)
		if dir[i].IsDir() {
			if err == filepath.SkipDir {
				err = nil
			} else if err := mfs.walk(filename, fn); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (mfs *memoryFs) Walk(root string, fn filepath.WalkFunc) error {
	stat, err := mfs.Lstat(root)
	if err != nil {
		return err
	}
	err = fn(strings.Join([]string{root, "."}, string(filepath.Separator)), stat, nil)
	if err != nil {
		return err
	}
	return mfs.walk(root, fn)
}

func (mfs *memoryFs) OpenGitIndex(path string) (*index.Index, error) {
	f, _ := mfs.Filesystem.Chroot(filepath.Join(path, ".git"))
	storage := filesystem.NewStorage(f, cache.NewObjectLRUDefault())
	i, err := storage.Index()
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (mfs *memoryFs) Open(path string) (io.ReadCloser, error) {
	return mfs.Filesystem.Open(path)
}

func (mfs *memoryFs) Readlink(path string) (string, error) {
	return mfs.Filesystem.Readlink(path)
}

func TestCopyCollectorOverwritesReadOnlyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	assert.NoError(t, os.Mkdir(src, 0o755))
	srcFile := filepath.Join(src, "pack.idx")
	assert.NoError(t, os.WriteFile(srcFile, []byte("first"), 0o600))
	assert.NoError(t, os.Chmod(srcFile, 0o444))

	fc := &FileCollector{
		Fs:        &DefaultFs{},
		SrcPath:   src,
		SrcPrefix: src + string(filepath.Separator),
		Handler:   &CopyCollector{DstDir: dst},
	}
	assert.NoError(t, filepath.Walk(src, fc.CollectFiles(context.Background(), nil)))

	assert.NoError(t, os.Chmod(srcFile, 0o600))
	assert.NoError(t, os.WriteFile(srcFile, []byte("second"), 0o600))
	assert.NoError(t, os.Chmod(srcFile, 0o444))

	assert.NoError(t, filepath.Walk(src, fc.CollectFiles(context.Background(), nil)))

	got, err := os.ReadFile(filepath.Join(dst, "pack.idx"))
	assert.NoError(t, err)
	assert.Equal(t, "second", string(got))
	st, err := os.Stat(filepath.Join(dst, "pack.idx"))
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0o444), st.Mode().Perm())
}

func TestCopyCollectorRejectsEscapingPaths(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst")
	outside := filepath.Join(dir, "outside")
	assert.NoError(t, os.Mkdir(outside, 0o755))
	fi := fakeFileInfo{name: "f", mode: 0o644}

	cc := &CopyCollector{DstDir: dst}
	for _, name := range []string{"../outside/evil", "a/../../outside/evil", "/abs/evil", ""} {
		err := cc.WriteFile(name, fi, "", strings.NewReader("x"))
		assert.Error(t, err, name)
	}
	_, err := os.Stat(filepath.Join(outside, "evil"))
	assert.True(t, os.IsNotExist(err))

	assert.NoError(t, cc.WriteFile("link", fakeFileInfo{name: "link", mode: os.ModeSymlink | 0o777}, outside, nil))
	assert.Error(t, cc.WriteFile("link/evil", fi, "", strings.NewReader("x")))
	_, err = os.Stat(filepath.Join(outside, "evil"))
	assert.True(t, os.IsNotExist(err))

	assert.NoError(t, cc.WriteFile("ok/nested/file", fi, "", strings.NewReader("x")))
	got, err := os.ReadFile(filepath.Join(dst, "ok", "nested", "file"))
	assert.NoError(t, err)
	assert.Equal(t, "x", string(got))
}

type fakeFileInfo struct {
	name string
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

func TestIgnoredTrackedfile(t *testing.T) {
	fs := memfs.New()
	_ = fs.MkdirAll("mygitrepo/.git", 0o777)
	dotgit, _ := fs.Chroot("mygitrepo/.git")
	worktree, _ := fs.Chroot("mygitrepo")
	repo, _ := git.Init(filesystem.NewStorage(dotgit, cache.NewObjectLRUDefault()), worktree)
	f, _ := worktree.Create(".gitignore")
	_, _ = f.Write([]byte(".*\n"))
	f.Close()
	// This file shouldn't be in the tar
	f, _ = worktree.Create(".env")
	_, _ = f.Write([]byte("test=val1\n"))
	f.Close()
	w, _ := repo.Worktree()
	// .gitignore is in the tar after adding it to the index
	_, _ = w.Add(".gitignore")

	tmpTar, _ := fs.Create("temp.tar")
	tw := tar.NewWriter(tmpTar)
	ps, _ := gitignore.ReadPatterns(worktree, []string{})
	ignorer := gitignore.NewMatcher(ps)
	fc := &FileCollector{
		Fs:        &memoryFs{Filesystem: fs},
		Ignorer:   ignorer,
		SrcPath:   "mygitrepo",
		SrcPrefix: "mygitrepo" + string(filepath.Separator),
		Handler: &TarCollector{
			TarWriter: tw,
		},
	}
	err := fc.Fs.Walk("mygitrepo", fc.CollectFiles(context.Background(), []string{}))
	assert.NoError(t, err, "successfully collect files")
	tw.Close()
	_, _ = tmpTar.Seek(0, io.SeekStart)
	tr := tar.NewReader(tmpTar)
	h, err := tr.Next()
	assert.NoError(t, err, "tar must not be empty")
	assert.Equal(t, ".gitignore", h.Name)
	_, err = tr.Next()
	assert.ErrorIs(t, err, io.EOF, "tar must only contain one element")
}

func TestSymlinks(t *testing.T) {
	fs := memfs.New()
	_ = fs.MkdirAll("mygitrepo/.git", 0o777)
	dotgit, _ := fs.Chroot("mygitrepo/.git")
	worktree, _ := fs.Chroot("mygitrepo")
	repo, _ := git.Init(filesystem.NewStorage(dotgit, cache.NewObjectLRUDefault()), worktree)
	// This file shouldn't be in the tar
	f, err := worktree.Create(".env")
	assert.NoError(t, err)
	_, err = f.Write([]byte("test=val1\n"))
	assert.NoError(t, err)
	f.Close()
	err = worktree.Symlink(".env", "test.env")
	assert.NoError(t, err)

	w, err := repo.Worktree()
	assert.NoError(t, err)

	// .gitignore is in the tar after adding it to the index
	_, err = w.Add(".env")
	assert.NoError(t, err)
	_, err = w.Add("test.env")
	assert.NoError(t, err)

	tmpTar, _ := fs.Create("temp.tar")
	tw := tar.NewWriter(tmpTar)
	ps, _ := gitignore.ReadPatterns(worktree, []string{})
	ignorer := gitignore.NewMatcher(ps)
	fc := &FileCollector{
		Fs:        &memoryFs{Filesystem: fs},
		Ignorer:   ignorer,
		SrcPath:   "mygitrepo",
		SrcPrefix: "mygitrepo" + string(filepath.Separator),
		Handler: &TarCollector{
			TarWriter: tw,
		},
	}
	err = fc.Fs.Walk("mygitrepo", fc.CollectFiles(context.Background(), []string{}))
	assert.NoError(t, err, "successfully collect files")
	tw.Close()
	_, _ = tmpTar.Seek(0, io.SeekStart)
	tr := tar.NewReader(tmpTar)
	h, err := tr.Next()
	files := map[string]tar.Header{}
	for err == nil {
		files[h.Name] = *h
		h, err = tr.Next()
	}

	assert.Equal(t, ".env", files[".env"].Name)
	assert.Equal(t, "test.env", files["test.env"].Name)
	assert.Equal(t, ".env", files["test.env"].Linkname)
	assert.ErrorIs(t, err, io.EOF, "tar must be read cleanly to EOF")
}
