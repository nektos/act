package runner

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	config "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/nektos/act/pkg/common"
)

type ActionCache interface {
	Fetch(ctx context.Context, cacheDir, url, ref, token string) (string, error)
	GetTarArchive(ctx context.Context, cacheDir, sha, includePrefix string) (io.ReadCloser, error)
}

type GoGitActionCache struct {
	Path string
}

func (c GoGitActionCache) Fetch(ctx context.Context, cacheDir, url, ref, token string) (string, error) {
	logger := common.Logger(ctx)

	gitPath := path.Join(c.Path, safeFilename(cacheDir)+".git")

	logger.Infof("GoGitActionCache fetch %s with ref %s at %s", url, ref, gitPath)

	gogitrepo, err := git.PlainInit(gitPath, true)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		logger.Debugf("GoGitActionCache cache hit %s with ref %s at %s", url, ref, gitPath)
		gogitrepo, err = git.PlainOpen(gitPath)
	}
	if err != nil {
		return "", fmt.Errorf("GoGitActionCache failed to open bare git %s with ref %s at %s: %w", url, ref, gitPath, err)
	}
	tmpBranch := make([]byte, 12)
	if _, err := rand.Read(tmpBranch); err != nil {
		return "", fmt.Errorf("GoGitActionCache failed to generate random tmp branch %s with ref %s at %s: %w", url, ref, gitPath, err)
	}
	branchName := hex.EncodeToString(tmpBranch)

	var auth transport.AuthMethod
	if token != "" {
		auth = &http.BasicAuth{
			Username: "token",
			Password: token,
		}
	}
	remote, err := gogitrepo.CreateRemoteAnonymous(&config.RemoteConfig{
		Name: "anonymous",
		URLs: []string{
			url,
		},
	})
	if err != nil {
		return "", fmt.Errorf("GoGitActionCache failed to create remote %s with ref %s at %s: %w", url, ref, gitPath, err)
	}
	defer func() {
		_ = gogitrepo.DeleteBranch(branchName)
	}()
	if err := remote.FetchContext(ctx, &git.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec(ref + ":" + branchName),
		},
		Auth:  auth,
		Force: true,
	}); err != nil {
		return "", fmt.Errorf("GoGitActionCache failed to fetch %s with ref %s at %s: %w", url, ref, gitPath, err)
	}
	hash, err := gogitrepo.ResolveRevision(plumbing.Revision(branchName))
	if err != nil {
		return "", fmt.Errorf("GoGitActionCache failed to resolve sha %s with ref %s at %s: %w", url, ref, gitPath, err)
	}
	logger.Infof("GoGitActionCache fetch %s with ref %s at %s resolved to %s", url, ref, gitPath, hash.String())
	return hash.String(), nil
}

type GitFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
	mode    fs.FileMode
}

// IsDir implements fs.FileInfo.
func (g *GitFileInfo) IsDir() bool {
	return g.isDir
}

// ModTime implements fs.FileInfo.
func (g *GitFileInfo) ModTime() time.Time {
	return g.modTime
}

// Mode implements fs.FileInfo.
func (g *GitFileInfo) Mode() fs.FileMode {
	return g.mode
}

// Name implements fs.FileInfo.
func (g *GitFileInfo) Name() string {
	return g.name
}

// Size implements fs.FileInfo.
func (g *GitFileInfo) Size() int64 {
	return g.size
}

// Sys implements fs.FileInfo.
func (g *GitFileInfo) Sys() any {
	return nil
}

func (c GoGitActionCache) GetTarArchive(ctx context.Context, cacheDir, sha, includePrefix string) (io.ReadCloser, error) {
	logger := common.Logger(ctx)

	gitPath := path.Join(c.Path, safeFilename(cacheDir)+".git")

	logger.Infof("GoGitActionCache get content %s with sha %s subpath %s at %s", cacheDir, sha, includePrefix, gitPath)

	gogitrepo, err := git.PlainOpen(gitPath)
	if err != nil {
		return nil, fmt.Errorf("GoGitActionCache failed to open bare git %s with sha %s subpath %s at %s: %w", cacheDir, sha, includePrefix, gitPath, err)
	}
	commit, err := gogitrepo.CommitObject(plumbing.NewHash(sha))
	if err != nil {
		return nil, fmt.Errorf("GoGitActionCache failed to get commit %s with sha %s subpath %s at %s: %w", cacheDir, sha, includePrefix, gitPath, err)
	}
	t, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("GoGitActionCache failed to open git tree %s with sha %s subpath %s at %s: %w", cacheDir, sha, includePrefix, gitPath, err)
	}
	files, err := commit.Files()
	if err != nil {
		return nil, fmt.Errorf("GoGitActionCache failed to list files %s with sha %s subpath %s at %s: %w", cacheDir, sha, includePrefix, gitPath, err)
	}
	rpipe, wpipe := io.Pipe()
	// Interrupt io.Copy using ctx
	ch := make(chan int, 1)
	go func() {
		select {
		case <-ctx.Done():
			wpipe.CloseWithError(ctx.Err())
		case <-ch:
		}
	}()
	go func() {
		defer wpipe.Close()
		defer close(ch)
		tw := tar.NewWriter(wpipe)
		cleanIncludePrefix := path.Clean(includePrefix)
		wpipe.CloseWithError(files.ForEach(func(f *object.File) error {
			return actionCacheCopyFileOrDir(ctx, cleanIncludePrefix, t, tw, f.Name, f)
		}))
	}()
	return rpipe, err
}

func actionCacheCopyFileOrDir(ctx context.Context, cleanIncludePrefix string, t *object.Tree, tw *tar.Writer, origin string, f *object.File) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name := origin
	if strings.HasPrefix(name, cleanIncludePrefix+"/") {
		name = name[len(cleanIncludePrefix)+1:]
	} else if cleanIncludePrefix != "." && name != cleanIncludePrefix {
		return nil
	}
	fmode, err := f.Mode.ToOSFileMode()
	if err != nil {
		return err
	}
	if fmode&fs.ModeSymlink == fs.ModeSymlink {
		content, err := f.Contents()
		if err != nil {
			return err
		}

		destPath := path.Join(path.Dir(f.Name), content)

		subtree, err := t.Tree(destPath)
		if err == nil {
			return subtree.Files().ForEach(func(ft *object.File) error {
				return actionCacheCopyFileOrDir(ctx, cleanIncludePrefix, t, tw, origin+strings.TrimPrefix(ft.Name, f.Name), f)
			})
		}

		f, err := t.File(destPath)
		if err != nil {
			return fmt.Errorf("%s (%s): %w", destPath, origin, err)
		}
		return actionCacheCopyFileOrDir(ctx, cleanIncludePrefix, t, tw, origin, f)
	}
	header, err := tar.FileInfoHeader(&GitFileInfo{
		name: name,
		mode: fmode,
		size: f.Size,
	}, "")
	if err != nil {
		return err
	}
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}
	reader, err := f.Reader()
	if err != nil {
		return err
	}
	_, err = io.Copy(tw, reader)
	return err
}
