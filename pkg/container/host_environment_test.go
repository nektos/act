package container

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Type assert HostEnvironment implements ExecutionsEnvironment
var _ ExecutionsEnvironment = &HostEnvironment{}

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

func TestCopyTarStreamRejectsEscapingEntries(t *testing.T) {
	dir := t.TempDir()
	e := &HostEnvironment{}
	for _, name := range []string{"../evil", "a/../../evil", "/evil"} {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		assert.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: 1, Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte("x"))
		assert.NoError(t, err)
		assert.NoError(t, tw.Close())

		err = e.CopyTarStream(context.Background(), filepath.Join(dir, "dest"), &buf)
		assert.Error(t, err, name)
	}
	_, err := os.Stat(filepath.Join(dir, "evil"))
	assert.True(t, os.IsNotExist(err))
}

func TestCopyRejectsEscapingNames(t *testing.T) {
	dir := t.TempDir()
	e := &HostEnvironment{}
	err := e.Copy(filepath.Join(dir, "dest"), &FileEntry{Name: "../evil", Mode: 0o644, Body: "x"})(context.Background())
	assert.Error(t, err)
	_, err = os.Stat(filepath.Join(dir, "evil"))
	assert.True(t, os.IsNotExist(err))

	assert.NoError(t, e.Copy(filepath.Join(dir, "dest"), &FileEntry{Name: "workflow/event.json", Mode: 0o644, Body: "{}"})(context.Background()))
	got, err := os.ReadFile(filepath.Join(dir, "dest", "workflow", "event.json"))
	assert.NoError(t, err)
	assert.Equal(t, "{}", string(got))
}
