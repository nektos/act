package container

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Type assert HostEnvironment implements ExecutionsEnvironment
var _ ExecutionsEnvironment = &HostEnvironment{}

type cancelAfterReader struct {
	reader    *bytes.Reader
	cancel    context.CancelFunc
	threshold int
	read      int
}

func (r *cancelAfterReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += n
	if r.read >= r.threshold {
		r.cancel()
	}
	return n, err
}

func TestCopyDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-host-env-*")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)
	ctx := context.Background()
	e := &HostEnvironment{
		Path:      filepath.Join(dir, "path"),
		TmpDir:    filepath.Join(dir, "tmp"),
		ToolCache: filepath.Join(dir, "tool_cache"),
		ActPath:   filepath.Join(dir, "act_path"),
		StdOut:    os.Stdout,
		Workdir:   path.Join("testdata", "scratch"),
	}
	_ = os.MkdirAll(e.Path, 0700)
	_ = os.MkdirAll(e.TmpDir, 0700)
	_ = os.MkdirAll(e.ToolCache, 0700)
	_ = os.MkdirAll(e.ActPath, 0700)
	err = e.CopyDir(e.Workdir, e.Path, true)(ctx)
	assert.NoError(t, err)
}

func TestCopyDirOverwritesReadOnlyGitPack(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	relativePack := filepath.Join(".git", "objects", "pack", "pack-test.idx")
	srcPack := filepath.Join(srcDir, relativePack)
	// CopyDir preserves the source directory's base name beneath destDir.
	destPack := filepath.Join(destDir, filepath.Base(srcDir), relativePack)
	require.NoError(t, os.MkdirAll(filepath.Dir(srcPack), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(destPack), 0o755))
	require.NoError(t, os.WriteFile(srcPack, []byte("new pack index"), 0o600))
	require.NoError(t, os.Chmod(srcPack, 0o444))
	require.NoError(t, os.WriteFile(destPack, []byte("old pack index"), 0o600))
	require.NoError(t, os.Chmod(destPack, 0o444))

	err := (&HostEnvironment{}).CopyDir(destDir, srcDir, false)(context.Background())
	require.NoError(t, err)
	content, err := os.ReadFile(destPack)
	require.NoError(t, err)
	assert.Equal(t, "new pack index", string(content))
	info, err := os.Stat(destPack)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o444), info.Mode().Perm())
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(destPack), ".act-copy-*"))
	require.NoError(t, err)
	assert.Empty(t, backups)
}

func TestCopyDirPreservesLongConfinedSymlinkChain(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "target"), []byte("confined"), 0o600))
	for i := 9; i >= 0; i-- {
		target := "target"
		if i < 9 {
			target = fmt.Sprintf("link-%d", i+1)
		}
		if err := os.Symlink(target, filepath.Join(srcDir, fmt.Sprintf("link-%d", i))); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	err := (&HostEnvironment{}).CopyDir(destDir, srcDir, false)(context.Background())
	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(destDir, filepath.Base(srcDir), "link-0"))
	require.NoError(t, err)
	assert.Equal(t, "confined", string(content))
}

func TestGetContainerArchive(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-host-env-*")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)
	ctx := context.Background()
	e := &HostEnvironment{
		Path:      filepath.Join(dir, "path"),
		TmpDir:    filepath.Join(dir, "tmp"),
		ToolCache: filepath.Join(dir, "tool_cache"),
		ActPath:   filepath.Join(dir, "act_path"),
		StdOut:    os.Stdout,
		Workdir:   path.Join("testdata", "scratch"),
	}
	_ = os.MkdirAll(e.Path, 0700)
	_ = os.MkdirAll(e.TmpDir, 0700)
	_ = os.MkdirAll(e.ToolCache, 0700)
	_ = os.MkdirAll(e.ActPath, 0700)
	expectedContent := []byte("sdde/7sh")
	err = os.WriteFile(filepath.Join(e.Path, "action.yml"), expectedContent, 0600)
	assert.NoError(t, err)
	archive, err := e.GetContainerArchive(ctx, e.Path)
	assert.NoError(t, err)
	defer archive.Close()
	reader := tar.NewReader(archive)
	h, err := reader.Next()
	assert.NoError(t, err)
	assert.Equal(t, "action.yml", h.Name)
	content, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, content)
	_, err = reader.Next()
	assert.ErrorIs(t, err, io.EOF)
}

func reverseOrderedSymlinkArchive(t *testing.T) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	assert.NoError(t, writer.WriteHeader(&tar.Header{
		Name:     "b",
		Linkname: "a/../outside",
		Mode:     0o777,
		Typeflag: tar.TypeSymlink,
	}))
	assert.NoError(t, writer.WriteHeader(&tar.Header{
		Name:     "a",
		Linkname: ".",
		Mode:     0o777,
		Typeflag: tar.TypeSymlink,
	}))
	assert.NoError(t, writer.Close())
	return archive.Bytes()
}

func TestCopyTarStreamCleansStagedSymlinkAfterTruncatedArchive(t *testing.T) {
	archive := reverseOrderedSymlinkArchive(t)
	destPath := filepath.Join(t.TempDir(), "destination")
	// Preserve the first complete header and leave an incomplete second header.
	err := (&HostEnvironment{}).CopyTarStream(context.Background(), destPath, bytes.NewReader(archive[:512+100]))
	assert.Error(t, err)
	_, err = os.Lstat(destPath)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestCopyTarStreamCleansStagedSymlinkAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	archive := reverseOrderedSymlinkArchive(t)
	reader := &cancelAfterReader{reader: bytes.NewReader(archive), cancel: cancel, threshold: 1024}
	destPath := filepath.Join(t.TempDir(), "destination")
	err := (&HostEnvironment{}).CopyTarStream(ctx, destPath, reader)
	assert.Error(t, err)
	_, err = os.Lstat(destPath)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestCopyDirFinalizesAfterWalkError(t *testing.T) {
	srcPath := t.TempDir()
	destPath := filepath.Join(t.TempDir(), "destination")
	links := []struct{ name, target string }{
		{"1-b", "2-a/../outside"},
		{"2-a", "."},
		{"3-bad", "../../outside"},
	}
	for _, link := range links {
		if err := os.Symlink(link.target, filepath.Join(srcPath, link.name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	err := (&HostEnvironment{}).CopyDir(destPath, srcPath, false)(context.Background())
	assert.Error(t, err)
	_, err = os.Lstat(filepath.Join(destPath, "1-b"))
	assert.ErrorIs(t, err, fs.ErrNotExist)
}
