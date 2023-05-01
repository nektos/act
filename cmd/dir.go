package cmd

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

var (
	UserHomeDir  string
	CacheHomeDir string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	UserHomeDir = home

	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		CacheHomeDir = v
	} else {
		CacheHomeDir = filepath.Join(UserHomeDir, ".cache")
	}
}
