package filecollector

import (
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareRootBackupRemovalClearsReadOnly(t *testing.T) {
	destDir := t.TempDir()
	root, err := os.OpenRoot(destDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	require.NoError(t, root.WriteFile("backup", []byte("old"), 0o444))
	require.NoError(t, root.Chmod("backup", 0o444))

	info, err := root.Lstat("backup")
	require.NoError(t, err)
	require.NoError(t, prepareRootBackupRemoval(root, "backup", info.Mode()))
	writable, err := root.Lstat("backup")
	require.NoError(t, err)
	assert.NotZero(t, writable.Mode().Perm()&fs.FileMode(0o200))
	require.NoError(t, root.Remove("backup"))
}
