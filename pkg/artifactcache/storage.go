package artifactcache

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

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

func (s *Storage) Exist(id uint64) (bool, error) {
	name := s.filename(id)
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Storage) Write(id uint64, offset int64, reader io.Reader) error {
	name := s.tempName(id, offset)
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func (s *Storage) Commit(id uint64, size int64) error {
	defer func() {
		_ = os.RemoveAll(s.tempDir(id))
	}()

	name := s.filename(id)
	tempNames, err := s.tempNames(id)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()

	var written int64
	for _, v := range tempNames {
		f, err := os.Open(v)
		if err != nil {
			return err
		}
		n, err := io.Copy(file, f)
		_ = f.Close()
		if err != nil {
			return err
		}
		written += n
	}

	if written != size {
		_ = file.Close()
		_ = os.Remove(name)
		return fmt.Errorf("broken file: %v != %v", written, size)
	}
	return nil
}

func (s *Storage) Serve(w http.ResponseWriter, r *http.Request, id uint64) {
	name := s.filename(id)
	http.ServeFile(w, r, name)
}

func (s *Storage) Remove(id uint64) {
	_ = os.Remove(s.filename(id))
	_ = os.RemoveAll(s.tempDir(id))
}

func (s *Storage) filename(id uint64) string {
	return filepath.Join(s.rootDir, fmt.Sprintf("%02x", id%0xff), fmt.Sprint(id))
}

func (s *Storage) tempDir(id uint64) string {
	return filepath.Join(s.rootDir, "tmp", fmt.Sprint(id))
}

func (s *Storage) tempName(id uint64, offset int64) string {
	return filepath.Join(s.tempDir(id), fmt.Sprintf("%016x", offset))
}

func (s *Storage) tempNames(id uint64) ([]string, error) {
	dir := s.tempDir(id)
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, v := range files {
		if !v.IsDir() {
			names = append(names, filepath.Join(dir, v.Name()))
		}
	}
	return names, nil
}
