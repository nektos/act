package runner

import (
	"context"
	"io"
	"path"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type GoGitActionCacheOfflineMode struct {
	Parent GoGitActionCache
}

func (c GoGitActionCacheOfflineMode) Fetch(ctx context.Context, cacheDir, url, ref, token string) (string, error) {
	sha, fetchErr := c.Parent.Fetch(ctx, cacheDir, url, ref, token)
	gitPath := path.Join(c.Parent.Path, safeFilename(cacheDir)+".git")
	gogitrepo, err := git.PlainOpen(gitPath)
	if err != nil {
		return "", fetchErr
	}
	refName := plumbing.ReferenceName("refs/action-cache-offline/" + ref)
	r, err := gogitrepo.Reference(refName, true)
	if fetchErr == nil {
		if err != nil || sha != r.Hash().String() {
			if err == nil {
				refName = r.Name()
			}
			ref := plumbing.NewHashReference(refName, plumbing.NewHash(sha))
			_ = gogitrepo.Storer.SetReference(ref)
		}
	} else if err == nil {
		return r.Hash().String(), nil
	}
	return sha, fetchErr
}

func (c GoGitActionCacheOfflineMode) GetTarArchive(ctx context.Context, cacheDir, sha, includePrefix string) (io.ReadCloser, error) {
	return c.Parent.GetTarArchive(ctx, cacheDir, sha, includePrefix)
}
