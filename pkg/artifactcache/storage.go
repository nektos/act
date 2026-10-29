package artifactcache

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

// Storage keeps cache blobs below rootDir. Every access goes through an
// os.Root, so a symlink placed in the cache directory cannot redirect a read
// or write outside it.
type Storage struct {
	rootDir string
}

func NewStorage(rootDir string) (*Storage, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, err
	}
	return &Storage{
		rootDir: rootDir,
	}, nil
}

func (s *Storage) open() (*os.Root, error) {
	if err := os.MkdirAll(s.rootDir, 0o755); err != nil {
		return nil, err
	}
	return os.OpenRoot(s.rootDir)
}

func (s *Storage) Exist(id uint64) (bool, error) {
	root, err := s.open()
	if err != nil {
		return false, err
	}
	defer root.Close()
	if _, err := root.Stat(s.filename(id)); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Storage) Write(id uint64, offset int64, reader io.Reader) error {
	root, err := s.open()
	if err != nil {
		return err
	}
	defer root.Close()
	name := s.tempName(id, offset)
	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	file, err := root.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func (s *Storage) Commit(id uint64, size int64) (int64, error) {
	root, err := s.open()
	if err != nil {
		return 0, err
	}
	defer root.Close()
	defer func() {
		_ = root.RemoveAll(s.tempDir(id))
	}()

	name := s.filename(id)
	tempNames, err := s.tempNames(root, id)
	if err != nil {
		return 0, err
	}

	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return 0, err
	}
	file, err := root.Create(name)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var written int64
	for _, v := range tempNames {
		f, err := root.Open(v)
		if err != nil {
			return 0, err
		}
		n, err := io.Copy(file, f)
		_ = f.Close()
		if err != nil {
			return 0, err
		}
		written += n
	}

	// If size is less than 0, it means the size is unknown.
	// We can't check the size of the file, just skip the check.
	// It happens when the request comes from old versions of actions, like `actions/cache@v2`.
	if size >= 0 && written != size {
		_ = file.Close()
		_ = root.Remove(name)
		return 0, fmt.Errorf("broken file: %v != %v", written, size)
	}

	return written, nil
}

func (s *Storage) Serve(w http.ResponseWriter, r *http.Request, id uint64) {
	root, err := s.open()
	if err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
	defer root.Close()
	f, err := root.Open(s.filename(id))
	if err != nil {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

func (s *Storage) Remove(id uint64) {
	root, err := s.open()
	if err != nil {
		return
	}
	defer root.Close()
	_ = root.Remove(s.filename(id))
	_ = root.RemoveAll(s.tempDir(id))
}

func (s *Storage) filename(id uint64) string {
	return filepath.Join(fmt.Sprintf("%02x", id%0xff), fmt.Sprint(id))
}

func (s *Storage) tempDir(id uint64) string {
	return filepath.Join("tmp", fmt.Sprint(id))
}

func (s *Storage) tempName(id uint64, offset int64) string {
	return filepath.Join(s.tempDir(id), fmt.Sprintf("%016x", offset))
}

func (s *Storage) tempNames(root *os.Root, id uint64) ([]string, error) {
	dir := s.tempDir(id)
	files, err := fs.ReadDir(root.FS(), filepath.ToSlash(dir))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, v := range files {
		if v.Type().IsRegular() {
			names = append(names, filepath.Join(dir, v.Name()))
		}
	}
	return names, nil
}
