package glob

import "os"

// FileAsset contains file information and path from globbing.
type FileAsset struct {
	os.FileInfo
	// Path to asset
	Path string
}

// Stat updates the stat of this asset.
func (fa *FileAsset) Stat() (*os.FileInfo, error) {
	fi, err := os.Stat(fa.Path)
	if err != nil {
		return nil, err
	}
	fa.FileInfo = fi
	return &fa.FileInfo, nil
}
