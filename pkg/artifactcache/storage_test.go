package artifactcache

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageDoesNotFollowSymlinksOutOfRoot(t *testing.T) {
	dir := t.TempDir()
	rootDir := filepath.Join(dir, "cache")
	outside := filepath.Join(dir, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "7"), []byte("secret"), 0o600))

	s, err := NewStorage(rootDir)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(outside, filepath.Join(rootDir, "07")))
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "tmp"), 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(rootDir, "tmp", "8")))

	ok, err := s.Exist(7)
	assert.Error(t, err)
	assert.False(t, ok)

	rr := httptest.NewRecorder()
	s.Serve(rr, httptest.NewRequest(http.MethodGet, "/", nil), 7)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NotContains(t, rr.Body.String(), "secret")

	assert.Error(t, s.Write(8, 0, strings.NewReader("evil")))
	_, err = os.Stat(filepath.Join(outside, "0000000000000000"))
	assert.True(t, os.IsNotExist(err))

	s.Remove(7)
	_, err = os.Stat(filepath.Join(outside, "7"))
	assert.NoError(t, err)
}

func TestStorageRoundTrip(t *testing.T) {
	s, err := NewStorage(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, s.Write(3, 0, strings.NewReader("hello ")))
	require.NoError(t, s.Write(3, 6, strings.NewReader("world")))
	n, err := s.Commit(3, 11)
	require.NoError(t, err)
	assert.Equal(t, int64(11), n)

	ok, err := s.Exist(3)
	require.NoError(t, err)
	assert.True(t, ok)

	rr := httptest.NewRecorder()
	s.Serve(rr, httptest.NewRequest(http.MethodGet, "/", nil), 3)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "hello world", rr.Body.String())

	s.Remove(3)
	ok, err = s.Exist(3)
	require.NoError(t, err)
	assert.False(t, ok)
}
