package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"

	"github.com/nektos/act/pkg/common"
)

var (
	codeCommitHTTPRegex = regexp.MustCompile(`^https?://git-codecommit\.(.+)\.amazonaws.com/v1/repos/(.+)$`)
	codeCommitSSHRegex  = regexp.MustCompile(`ssh://git-codecommit\.(.+)\.amazonaws.com/v1/repos/(.+)$`)
	githubHTTPRegex     = regexp.MustCompile(`^https?://.*github.com.*/(.+)/(.+?)(?:.git)?$`)
	githubSSHRegex      = regexp.MustCompile(`github.com[:/](.+)/(.+?)(?:.git)?$`)

	cloneLock sync.Mutex

	ErrShortRef = errors.New("short SHA references are not supported")
	ErrNoRepo   = errors.New("unable to find git repo")
)

type Error struct {
	err    error
	commit string
}

func (e *Error) Error() string {
	return e.err.Error()
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) Commit() string {
	return e.commit
}

// FindGitRevision get the current git revision
func FindGitRevision(ctx context.Context, file string) (shortSha string, sha string, err error) {
	logger := common.Logger(ctx)

	gitDir, err := git.PlainOpenWithOptions(
		file,
		&git.PlainOpenOptions{
			DetectDotGit:          true,
			EnableDotGitCommonDir: true,
		},
	)

	if err != nil {
		logger.WithError(err).Error("path", file, "not located inside a git repository")
		return "", "", err
	}

	head, err := gitDir.Reference(plumbing.HEAD, true)
	if err != nil {
		return "", "", err
	}

	if head.Hash().IsZero() {
		return "", "", fmt.Errorf("HEAD sha1 could not be resolved")
	}

	hash := head.Hash().String()

	logger.Debugf("Found revision: %s", hash)
	return hash[:7], strings.TrimSpace(hash), nil
}

// FindGitRef get the current git ref
func FindGitRef(ctx context.Context, file string) (string, error) {
	logger := common.Logger(ctx)

	logger.Debugf("Loading revision from git directory")
	_, ref, err := FindGitRevision(ctx, file)
	if err != nil {
		return "", err
	}

	logger.Debugf("HEAD points to '%s'", ref)

	// Prefer the git library to iterate over the references and find a matching tag or branch.
	var refTag = ""
	var refBranch = ""
	repo, err := git.PlainOpenWithOptions(
		file,
		&git.PlainOpenOptions{
			DetectDotGit:          true,
			EnableDotGitCommonDir: true,
		},
	)

	if err != nil {
		return "", err
	}

	iter, err := repo.References()
	if err != nil {
		return "", err
	}

	// find the reference that matches the revision's has
	err = iter.ForEach(func(r *plumbing.Reference) error {
		/* tags and branches will have the same hash
		 * when a user checks out a tag, it is not mentioned explicitly
		 * in the go-git package, we must identify the revision
		 * then check if any tag matches that revision,
		 * if so then we checked out a tag
		 * else we look for branches and if matches,
		 * it means we checked out a branch
		 *
		 * If a branches matches first we must continue and check all tags (all references)
		 * in case we match with a tag later in the interation
		 */
		if r.Hash().String() == ref {
			if r.Name().IsTag() {
				refTag = r.Name().String()
			}
			if r.Name().IsBranch() {
				refBranch = r.Name().String()
			}
		}

		// we found what we where looking for
		if refTag != "" && refBranch != "" {
			return storer.ErrStop
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	// order matters here see above comment.
	if refTag != "" {
		return refTag, nil
	}
	if refBranch != "" {
		return refBranch, nil
	}

	return "", fmt.Errorf("failed to identify reference (tag/branch) for the checked-out revision '%s'", ref)
}

// FindGithubRepo get the repo
func FindGithubRepo(ctx context.Context, file, githubInstance, remoteName string) (string, error) {
	if remoteName == "" {
		remoteName = "origin"
	}

	url, err := findGitRemoteURL(ctx, file, remoteName)
	if err != nil {
		return "", err
	}
	_, slug, err := findGitSlug(url, githubInstance)
	return slug, err
}

func findGitRemoteURL(_ context.Context, file, remoteName string) (string, error) {
	repo, err := git.PlainOpenWithOptions(
		file,
		&git.PlainOpenOptions{
			DetectDotGit:          true,
			EnableDotGitCommonDir: true,
		},
	)
	if err != nil {
		return "", err
	}

	remote, err := repo.Remote(remoteName)
	if err != nil {
		return "", err
	}

	if len(remote.Config().URLs) < 1 {
		return "", fmt.Errorf("remote '%s' exists but has no URL", remoteName)
	}

	return remote.Config().URLs[0], nil
}

func findGitSlug(url string, githubInstance string) (string, string, error) {
	if matches := codeCommitHTTPRegex.FindStringSubmatch(url); matches != nil {
		return "CodeCommit", matches[2], nil
	} else if matches := codeCommitSSHRegex.FindStringSubmatch(url); matches != nil {
		return "CodeCommit", matches[2], nil
	} else if matches := githubHTTPRegex.FindStringSubmatch(url); matches != nil {
		return "GitHub", fmt.Sprintf("%s/%s", matches[1], matches[2]), nil
	} else if matches := githubSSHRegex.FindStringSubmatch(url); matches != nil {
		return "GitHub", fmt.Sprintf("%s/%s", matches[1], matches[2]), nil
	} else if githubInstance != "github.com" {
		gheHTTPRegex := regexp.MustCompile(fmt.Sprintf(`^https?://%s/(.+)/(.+?)(?:.git)?$`, githubInstance))
		gheSSHRegex := regexp.MustCompile(fmt.Sprintf(`%s[:/](.+)/(.+?)(?:.git)?$`, githubInstance))
		if matches := gheHTTPRegex.FindStringSubmatch(url); matches != nil {
			return "GitHubEnterprise", fmt.Sprintf("%s/%s", matches[1], matches[2]), nil
		} else if matches := gheSSHRegex.FindStringSubmatch(url); matches != nil {
			return "GitHubEnterprise", fmt.Sprintf("%s/%s", matches[1], matches[2]), nil
		}
	}
	return "", url, nil
}

// NewGitCloneExecutorInput the input for the NewGitCloneExecutor
type NewGitCloneExecutorInput struct {
	URL         string
	Ref         string
	Dir         string
	Token       string
	OfflineMode bool
}

// CloneIfRequired ...
func CloneIfRequired(ctx context.Context, refName plumbing.ReferenceName, input NewGitCloneExecutorInput, logger log.FieldLogger) (*git.Repository, error) {
	r, err := git.PlainOpen(input.Dir)
	if err != nil {
		var progressWriter io.Writer
		if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			if entry, ok := logger.(*log.Entry); ok {
				progressWriter = entry.WriterLevel(log.DebugLevel)
			} else if lgr, ok := logger.(*log.Logger); ok {
				progressWriter = lgr.WriterLevel(log.DebugLevel)
			} else {
				log.Errorf("Unable to get writer from logger (type=%T)", logger)
				progressWriter = os.Stdout
			}
		}

		cloneOptions := git.CloneOptions{
			URL:      input.URL,
			Progress: progressWriter,
		}
		if input.Token != "" {
			cloneOptions.Auth = &http.BasicAuth{
				Username: "token",
				Password: input.Token,
			}
		}

		r, err = git.PlainCloneContext(ctx, input.Dir, false, &cloneOptions)
		if err != nil {
			logger.Errorf("Unable to clone %v %s: %v", input.URL, refName, err)
			return nil, err
		}

		if err = os.Chmod(input.Dir, 0o755); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func gitOptions(token string) (fetchOptions git.FetchOptions, pullOptions git.PullOptions) {
	fetchOptions.RefSpecs = []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"}
	pullOptions.Force = true

	if token != "" {
		auth := &http.BasicAuth{
			Username: "token",
			Password: token,
		}
		fetchOptions.Auth = auth
		pullOptions.Auth = auth
	}

	return fetchOptions, pullOptions
}

// NewGitCloneExecutor creates an executor to clone git repos
//
//nolint:gocyclo
func NewGitCloneExecutor(input NewGitCloneExecutorInput) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		logger.Infof("  \u2601  git clone '%s' # ref=%s", input.URL, input.Ref)
		logger.Debugf("  cloning %s to %s", input.URL, input.Dir)

		cloneLock.Lock()
		defer cloneLock.Unlock()

		refName := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", input.Ref))
		r, err := CloneIfRequired(ctx, refName, input, logger)
		if err != nil {
			return err
		}

		isOfflineMode := input.OfflineMode

		// fetch latest changes
		fetchOptions, pullOptions := gitOptions(input.Token)

		if !isOfflineMode {
			err = r.Fetch(&fetchOptions)
			if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
				return err
			}
		}

		var hash *plumbing.Hash
		rev := plumbing.Revision(input.Ref)
		if hash, err = r.ResolveRevision(rev); err != nil {
			logger.Errorf("Unable to resolve %s: %v", input.Ref, err)
		}

		if hash.String() != input.Ref && strings.HasPrefix(hash.String(), input.Ref) {
			return &Error{
				err:    ErrShortRef,
				commit: hash.String(),
			}
		}

		// At this point we need to know if it's a tag or a branch
		// And the easiest way to do it is duck typing
		//
		// If err is nil, it's a tag so let's proceed with that hash like we would if
		// it was a sha
		refType := "tag"
		rev = plumbing.Revision(path.Join("refs", "tags", input.Ref))
		if _, err := r.Tag(input.Ref); errors.Is(err, git.ErrTagNotFound) {
			rName := plumbing.ReferenceName(path.Join("refs", "remotes", "origin", input.Ref))
			if _, err := r.Reference(rName, false); errors.Is(err, plumbing.ErrReferenceNotFound) {
				refType = "sha"
				rev = plumbing.Revision(input.Ref)
			} else {
				refType = "branch"
				rev = plumbing.Revision(rName)
			}
		}

		if hash, err = r.ResolveRevision(rev); err != nil {
			logger.Errorf("Unable to resolve %s: %v", input.Ref, err)
			return err
		}

		var w *git.Worktree
		if w, err = r.Worktree(); err != nil {
			return err
		}

		// If the hash resolved doesn't match the ref provided in a workflow then we're
		// using a branch or tag ref, not a sha
		//
		// Repos on disk point to commit hashes, and need to checkout input.Ref before
		// we try and pull down any changes
		if hash.String() != input.Ref && refType == "branch" {
			logger.Debugf("Provided ref is not a sha. Checking out branch before pulling changes")
			sourceRef := plumbing.ReferenceName(path.Join("refs", "remotes", "origin", input.Ref))
			if err = w.Checkout(&git.CheckoutOptions{
				Branch: sourceRef,
				Force:  true,
			}); err != nil {
				logger.Errorf("Unable to checkout %s: %v", sourceRef, err)
				return err
			}
		}
		if !isOfflineMode {
			if err = w.Pull(&pullOptions); err != nil && err != git.NoErrAlreadyUpToDate {
				logger.Debugf("Unable to pull %s: %v", refName, err)
			}
		}
		logger.Debugf("Cloned %s to %s", input.URL, input.Dir)

		if hash.String() != input.Ref && refType == "branch" {
			logger.Debugf("Provided ref is not a sha. Updating branch ref after pull")
			if hash, err = r.ResolveRevision(rev); err != nil {
				logger.Errorf("Unable to resolve %s: %v", input.Ref, err)
				return err
			}
		}
		if err = w.Checkout(&git.CheckoutOptions{
			Hash:  *hash,
			Force: true,
		}); err != nil {
			logger.Errorf("Unable to checkout %s: %v", *hash, err)
			return err
		}

		if err = w.Reset(&git.ResetOptions{
			Mode:   git.HardReset,
			Commit: *hash,
		}); err != nil {
			logger.Errorf("Unable to reset to %s: %v", hash.String(), err)
			return err
		}

		logger.Debugf("Checked out %s", input.Ref)
		return nil
	}
}
