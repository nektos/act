package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	goURL "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/filecollector"
)

type LocalRepositoryCache struct {
	Parent            ActionCache
	LocalRepositories map[string]string
	CacheDirCache     map[string]string
	cacheDirMu        sync.RWMutex
}

func (l *LocalRepositoryCache) Fetch(ctx context.Context, cacheDir, url, ref, token string) (string, error) {
	logger := common.Logger(ctx)
	logger.Debugf("LocalRepositoryCache fetch %s with ref %s", url, ref)
	if dest, ok := l.LocalRepositories[fmt.Sprintf("%s@%s", url, ref)]; ok {
		logger.Infof("LocalRepositoryCache matched %s with ref %s to %s", url, ref, dest)
		l.cacheDirMu.Lock()
		l.CacheDirCache[fmt.Sprintf("%s@%s", cacheDir, ref)] = dest
		l.cacheDirMu.Unlock()
		return ref, nil
	}
	if purl, err := goURL.Parse(url); err == nil {
		if dest, ok := l.LocalRepositories[fmt.Sprintf("%s@%s", strings.TrimPrefix(purl.Path, "/"), ref)]; ok {
			logger.Infof("LocalRepositoryCache matched %s with ref %s to %s", url, ref, dest)
			l.cacheDirMu.Lock()
			l.CacheDirCache[fmt.Sprintf("%s@%s", cacheDir, ref)] = dest
			l.cacheDirMu.Unlock()
			return ref, nil
		}
	}
	logger.Infof("LocalRepositoryCache not matched %s with Ref %s", url, ref)
	return l.Parent.Fetch(ctx, cacheDir, url, ref, token)
}

func (l *LocalRepositoryCache) GetTarArchive(ctx context.Context, cacheDir, sha, includePrefix string) (io.ReadCloser, error) {
	logger := common.Logger(ctx)
	// sha is mapped to ref in fetch if there is a local override
	l.cacheDirMu.RLock()
	dest, ok := l.CacheDirCache[fmt.Sprintf("%s@%s", cacheDir, sha)]
	l.cacheDirMu.RUnlock()
	if ok {
		logger.Infof("LocalRepositoryCache read cachedir %s with ref %s and subpath '%s' from %s", cacheDir, sha, includePrefix, dest)
		srcPath := filepath.Join(dest, includePrefix)
		buf := &bytes.Buffer{}
		tw := tar.NewWriter(buf)
		defer tw.Close()
		srcPath = filepath.Clean(srcPath)
		fi, err := os.Lstat(srcPath)
		if err != nil {
			return nil, err
		}
		tc := &filecollector.TarCollector{
			TarWriter: tw,
		}
		if fi.IsDir() {
			srcPrefix := srcPath
			if !strings.HasSuffix(srcPrefix, string(filepath.Separator)) {
				srcPrefix += string(filepath.Separator)
			}
			fc := &filecollector.FileCollector{
				Fs:        &filecollector.DefaultFs{},
				SrcPath:   srcPath,
				SrcPrefix: srcPrefix,
				Handler:   tc,
			}
			err = filepath.Walk(srcPath, fc.CollectFiles(ctx, []string{}))
			if err != nil {
				return nil, err
			}
		} else {
			var f io.ReadCloser
			var linkname string
			if fi.Mode()&fs.ModeSymlink != 0 {
				linkname, err = os.Readlink(srcPath)
				if err != nil {
					return nil, err
				}
			} else {
				f, err = os.Open(srcPath)
				if err != nil {
					return nil, err
				}
				defer f.Close()
			}
			err := tc.WriteFile(fi.Name(), fi, linkname, f)
			if err != nil {
				return nil, err
			}
		}
		return io.NopCloser(buf), nil
	}
	logger.Infof("LocalRepositoryCache not matched cachedir %s with Ref %s and subpath '%s'", cacheDir, sha, includePrefix)
	return l.Parent.GetTarArchive(ctx, cacheDir, sha, includePrefix)
}
