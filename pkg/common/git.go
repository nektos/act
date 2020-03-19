package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"

	log "github.com/sirupsen/logrus"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

var (
	codeCommitHTTPRegex = regexp.MustCompile(`^https?://git-codecommit\.(.+)\.amazonaws.com/v1/repos/(.+)$`)
	codeCommitSSHRegex  = regexp.MustCompile(`ssh://git-codecommit\.(.+)\.amazonaws.com/v1/repos/(.+)$`)
	githubHTTPRegex     = regexp.MustCompile(`^https?://.*github.com.*/(.+)/(.+?)(?:.git)?$`)
	githubSSHRegex      = regexp.MustCompile(`github.com[:/](.+)/(.+).git$`)

	cloneLock sync.Mutex
)

func FindGitRepository(file string) (*git.Repository, error) {
	log.Debugf("Looking for a git repository in: %s", file)
	repository, err := git.PlainOpenWithOptions(file, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, err
	}

	return repository, nil
}

// FindGitRevision get the current git revision
func FindGitRevision(repository *git.Repository) (shortSha string, sha string, err error) {
	head, err := repository.Head()
	if err != nil {
		return "", "", err
	}

	hash := head.Hash().String()
	log.Debugf("Found revision: %s", hash)
	return hash[:7], hash, nil
}

// FindGitRef get the current git ref
func FindGitRef(repository *git.Repository) (string, error) {
	head, err := repository.Head()
	if err != nil {
		return "", err
	}

	tags, err := repository.Tags()
	if err != nil {
		return "", err
	}

	tagRefs := make(chan *plumbing.Reference)
	go func() {
		err := tags.ForEach(func(ref *plumbing.Reference) (err error) {
			if ref.Hash() == head.Hash() {
				tagRefs <- ref
			}
			return
		})
		if err != nil {
			log.Fatal(err)
		}

		close(tagRefs)
	}()

	for tagRef := range tagRefs {
		return tagRef.Name().String(), nil
	}

	return head.Name().String(), nil
}

// FindGithubRepo get the repo
func FindGithubRepoName(repository *git.Repository) (string, error) {
	url, err := findGitRemoteURL(repository)
	if err != nil {
		return "", err
	}
	_, slug, err := findGitSlug(url)
	return slug, err
}

func findGitRemoteURL(repository *git.Repository) (string, error) {
	remote, err := repository.Remote("origin")
	if err != nil {
		return "", err
	}
	url := remote.Config().URLs[0]
	return url, nil
}

func findGitSlug(url string) (string, string, error) {
	if matches := codeCommitHTTPRegex.FindStringSubmatch(url); matches != nil {
		return "CodeCommit", matches[2], nil
	} else if matches := codeCommitSSHRegex.FindStringSubmatch(url); matches != nil {
		return "CodeCommit", matches[2], nil
	} else if matches := githubHTTPRegex.FindStringSubmatch(url); matches != nil {
		return "GitHub", fmt.Sprintf("%s/%s", matches[1], matches[2]), nil
	} else if matches := githubSSHRegex.FindStringSubmatch(url); matches != nil {
		return "GitHub", fmt.Sprintf("%s/%s", matches[1], matches[2]), nil
	}
	return "", url, nil
}

// NewGitCloneExecutorInput the input for the NewGitCloneExecutor
type NewGitCloneExecutorInput struct {
	URL string
	Ref string
	Dir string
}

// NewGitCloneExecutor creates an executor to clone git repos
func NewGitCloneExecutor(input NewGitCloneExecutorInput) Executor {
	return func(ctx context.Context) error {
		logger := Logger(ctx)
		logger.Infof("  \u2601  git clone '%s' # ref=%s", input.URL, input.Ref)
		logger.Debugf("  cloning %s to %s", input.URL, input.Dir)

		cloneLock.Lock()
		defer cloneLock.Unlock()

		refName := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", input.Ref))

		r, err := git.PlainOpen(input.Dir)
		if err != nil {
			var progressWriter io.Writer
			if entry, ok := logger.(*log.Entry); ok {
				progressWriter = entry.WriterLevel(log.DebugLevel)
			} else if lgr, ok := logger.(*log.Logger); ok {
				progressWriter = lgr.WriterLevel(log.DebugLevel)
			} else {
				log.Errorf("Unable to get writer from logger (type=%T)", logger)
				progressWriter = os.Stdout
			}

			r, err = git.PlainClone(input.Dir, false, &git.CloneOptions{
				URL:      input.URL,
				Progress: progressWriter,
				//ReferenceName: refName,
			})
			if err != nil {
				logger.Errorf("Unable to clone %v %s: %v", input.URL, refName, err)
				return err
			}
			_ = os.Chmod(input.Dir, 0755)
		}

		w, err := r.Worktree()
		if err != nil {
			return err
		}

		err = w.Pull(&git.PullOptions{
			//ReferenceName: refName,
			Force: true,
		})
		if err != nil && err.Error() != "already up-to-date" {
			logger.Debugf("Unable to pull %s: %v", refName, err)
		}
		logger.Debugf("Cloned %s to %s", input.URL, input.Dir)

		hash, err := r.ResolveRevision(plumbing.Revision(input.Ref))
		if err != nil {
			logger.Errorf("Unable to resolve %s: %v", input.Ref, err)
			return err
		}

		err = w.Checkout(&git.CheckoutOptions{
			//Branch: refName,
			Hash:  *hash,
			Force: true,
		})
		if err != nil {
			logger.Errorf("Unable to checkout %s: %v", *hash, err)
			return err
		}

		err = w.Reset(&git.ResetOptions{
			Mode:   git.HardReset,
			Commit: *hash,
		})
		if err != nil {
			logger.Errorf("Unable to reset to %s: %v", hash.String(), err)
			return err
		}

		logger.Debugf("Checked out %s", input.Ref)
		return nil
	}
}
