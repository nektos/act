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

	git "github.com/go-git/go-git/v5"
	config "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type ActionCache interface {
	Fetch(ctx context.Context, cacheDir, url, ref, token string) (string, error)
	GetTarArchive(ctx context.Context, cacheDir, sha, includePrefix string) (io.ReadCloser, error)
}

type GoGitActionCache struct {
	Path string
}

func (c GoGitActionCache) Fetch(ctx context.Context, cacheDir, url, ref, token string) (string, error) {
	gitPath := path.Join(c.Path, safeFilename(cacheDir)+".git")
	gogitrepo, err := git.PlainInit(gitPath, true)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		gogitrepo, err = git.PlainOpen(gitPath)
	}
	if err != nil {
		return "", err
	}
	tmpBranch := make([]byte, 12)
	if _, err := rand.Read(tmpBranch); err != nil {
		return "", err
	}
	branchName := hex.EncodeToString(tmpBranch)
	var refSpec config.RefSpec
	spec := config.RefSpec(ref + ":" + branchName)
	tagOrSha := false
	if spec.IsExactSHA1() {
		refSpec = spec
	} else if strings.HasPrefix(ref, "refs/") {
		refSpec = config.RefSpec(ref + ":refs/heads/" + branchName)
	} else {
		tagOrSha = true
		refSpec = config.RefSpec("refs/*/" + ref + ":refs/heads/*/" + branchName)
	}
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
		return "", err
	}
	defer func() {
		if refs, err := gogitrepo.References(); err == nil {
			_ = refs.ForEach(func(r *plumbing.Reference) error {
				if strings.Contains(r.Name().String(), branchName) {
					return gogitrepo.DeleteBranch(r.Name().String())
				}
				return nil
			})
		}
	}()
	if err := remote.FetchContext(ctx, &git.FetchOptions{
		RefSpecs: []config.RefSpec{
			refSpec,
		},
		Auth:  auth,
		Force: true,
	}); err != nil {
		if tagOrSha && errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "", fmt.Errorf("couldn't find remote ref \"%s\"", ref)
		}
		return "", err
	}
	if tagOrSha {
		for _, prefix := range []string{"refs/heads/tags/", "refs/heads/heads/"} {
			hash, err := gogitrepo.ResolveRevision(plumbing.Revision(prefix + branchName))
			if err == nil {
				return hash.String(), nil
			}
		}
	}
	hash, err := gogitrepo.ResolveRevision(plumbing.Revision(branchName))
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}

func (c GoGitActionCache) GetTarArchive(ctx context.Context, cacheDir, sha, includePrefix string) (io.ReadCloser, error) {
	gitPath := path.Join(c.Path, safeFilename(cacheDir)+".git")
	gogitrepo, err := git.PlainOpen(gitPath)
	if err != nil {
		return nil, err
	}
	commit, err := gogitrepo.CommitObject(plumbing.NewHash(sha))
	if err != nil {
		return nil, err
	}
	files, err := commit.Files()
	if err != nil {
		return nil, err
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
			if err := ctx.Err(); err != nil {
				return err
			}
			name := f.Name
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
				return tw.WriteHeader(&tar.Header{
					Name:     name,
					Mode:     int64(fmode),
					Linkname: content,
				})
			}
			err = tw.WriteHeader(&tar.Header{
				Name: name,
				Mode: int64(fmode),
				Size: f.Size,
			})
			if err != nil {
				return err
			}
			reader, err := f.Reader()
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, reader)
			return err
		}))
	}()
	return rpipe, err
}
