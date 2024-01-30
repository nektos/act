package container

import (
	"archive/tar"
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
