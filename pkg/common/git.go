package common

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-ini/ini"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var (
	codeCommitHTTPRegex = regexp.MustCompile(`^https?://git-codecommit\.(.+)\.amazonaws.com/v1/repos/(.+)$`)
	codeCommitSSHRegex  = regexp.MustCompile(`ssh://git-codecommit\.(.+)\.amazonaws.com/v1/repos/(.+)$`)
	githubHTTPRegex     = regexp.MustCompile(`^https?://.*github.com.*/(.+)/(.+?)(?:.git)?$`)
	githubSSHRegex      = regexp.MustCompile(`github.com[:/](.+)/(.+?)(?:.git)?$`)

	cloneLock sync.Mutex
)

// FindGitRevision get the current git revision
func FindGitRevision(file string) (shortSha string, sha string, err error) {
	gitDir, err := findGitDirectory(file)
	if err != nil {
		return "", "", err
	}

	bts, err := ioutil.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", "", err
	}

	var ref = strings.TrimSpace(strings.TrimPrefix(string(bts), "ref:"))
	var refBuf []byte
	if strings.HasPrefix(ref, "refs/") {
		// load commitid ref
		refBuf, err = ioutil.ReadFile(filepath.Join(gitDir, ref))
		if err != nil {
			return "", "", err
		}
	} else {
		refBuf = []byte(ref)
	}

	log.Tracef("Found revision: %s", refBuf)
	return string(refBuf[:7]), strings.TrimSpace(string(refBuf)), nil
}

// FindGitRef get the current git ref
func FindGitRef(file string) (string, error) {
	gitDir, err := findGitDirectory(file)
	if err != nil {
		return "", err
	}
	log.Tracef("Loading revision from git directory '%s'", gitDir)

	_, ref, err := FindGitRevision(file)
	if err != nil {
		return "", err
	}

	log.Tracef("HEAD points to '%s'", ref)

	// Prefer the git library to iterate over the references and find a matching tag or branch.
	var refTag = ""
	var refBranch = ""
	r, err := git.PlainOpen(filepath.Join(gitDir, ".."))
	if err == nil {
		iter, err := r.References()
		if err == nil {
			for {
				r, err := iter.Next()
				if r == nil || err != nil {
					break
				}
				log.Tracef("Reference: name=%s sha=%s", r.Name().String(), r.Hash().String())
				if r.Hash().String() == ref {
					if r.Name().IsTag() {
						refTag = r.Name().String()
					}
					if r.Name().IsBranch() {
						refBranch = r.Name().String()
					}
				}
			}
			iter.Close()
		}
	}
	if refTag != "" {
		return refTag, nil
	}
	if refBranch != "" {
		return refBranch, nil
	}

	// If the above doesn't work, fall back to the old way

	// try tags first
	tag, err := findGitPrettyRef(ref, gitDir, "refs/tags")
	if err != nil || tag != "" {
		return tag, err
	}
	// and then branches
	return findGitPrettyRef(ref, gitDir, "refs/heads")
}

func findGitPrettyRef(head, root, sub string) (string, error) {
	var name string
	var err = filepath.Walk(filepath.Join(root, sub), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if name != "" || info.IsDir() {
			return nil
		}
		var bts []byte
		if bts, err = ioutil.ReadFile(path); err != nil {
			return err
		}
		var pointsTo = strings.TrimSpace(string(bts))
		if head == pointsTo {
			// On Windows paths are separated with backslash character so they should be replaced to provide proper git refs format
			name = strings.TrimPrefix(strings.ReplaceAll(strings.Replace(path, root, "", 1), `\`, `/`), "/")
			log.Tracef("HEAD matches %s", name)
		}
		return nil
	})
	return name, err
}

// FindGithubRepo get the repo
func FindGithubRepo(file string, githubInstance string) (string, error) {
	url, err := findGitRemoteURL(file)
	if err != nil {
		return "", err
	}
	_, slug, err := findGitSlug(url, githubInstance)
	return slug, err
}

func findGitRemoteURL(file string) (string, error) {
	gitDir, err := findGitDirectory(file)
	if err != nil {
		return "", err
	}
	log.Tracef("Loading slug from git directory '%s'", gitDir)

	gitconfig, err := ini.InsensitiveLoad(fmt.Sprintf("%s/config", gitDir))
	if err != nil {
		return "", err
	}
	remote, err := gitconfig.GetSection("remote \"origin\"")
	if err != nil {
		return "", err
	}
	urlKey, err := remote.GetKey("url")
	if err != nil {
		return "", err
	}
	url := urlKey.String()
	return url, nil
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

func findGitDirectory(fromFile string) (string, error) {
	absPath, err := filepath.Abs(fromFile)
	if err != nil {
		return "", err
	}

	fi, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}

	var dir string
	if fi.Mode().IsDir() {
		dir = absPath
	} else {
		dir = filepath.Dir(absPath)
	}

	gitPath := filepath.Join(dir, ".git")
	fi, err = os.Stat(gitPath)
	if err == nil && fi.Mode().IsDir() {
		return gitPath, nil
	} else if dir == "/" || dir == "C:\\" || dir == "c:\\" {
		return "", errors.New("unable to find git repo")
	}

	return findGitDirectory(filepath.Dir(dir))
}

// NewGitCloneExecutorInput the input for the NewGitCloneExecutor
type NewGitCloneExecutorInput struct {
	URL   string
	Ref   string
	Dir   string
	Token string
}

// CloneIfRequired ...
func CloneIfRequired(ctx context.Context, refName plumbing.ReferenceName, input NewGitCloneExecutorInput, logger log.Ext1FieldLogger) (*git.Repository, error) {
	r, err := git.PlainOpen(input.Dir)
	if err != nil {
		var progressWriter io.Writer
		if CheckIfTerminal(os.Stdout) {
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

		if err = os.Chmod(input.Dir, 0755); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// NewGitCloneExecutor creates an executor to clone git repos
// nolint:gocyclo
func NewGitCloneExecutor(input NewGitCloneExecutorInput) Executor {
	return func(ctx context.Context) error {
		logger := Logger(ctx)
		logger.WithField("emoji", "  \u2601").Infof("  git clone '%s' # ref=%s", input.URL, input.Ref)
		logger.Tracef("  cloning %s to %s", input.URL, input.Dir)

		cloneLock.Lock()
		defer cloneLock.Unlock()

		refName := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", input.Ref))
		r, err := CloneIfRequired(ctx, refName, input, logger)
		if err != nil {
			return err
		}

		// fetch latest changes
		fetchOptions := git.FetchOptions{
			RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
		}
		if input.Token != "" {
			fetchOptions.Auth = &http.BasicAuth{
				Username: "token",
				Password: input.Token,
			}
		}

		err = r.Fetch(&fetchOptions)
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return err
		}

		var hash *plumbing.Hash
		rev := plumbing.Revision(input.Ref)
		if hash, err = r.ResolveRevision(rev); err != nil {
			logger.Errorf("Unable to resolve %s: %v", input.Ref, err)
		}

		if hash.String() != input.Ref && strings.HasPrefix(hash.String(), input.Ref) {
			return errors.Wrap(errors.New(hash.String()), "short SHA references are not supported")
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
			logger.Tracef("Provided ref is not a sha. Checking out branch before pulling changes")
			sourceRef := plumbing.ReferenceName(path.Join("refs", "remotes", "origin", input.Ref))
			if err = w.Checkout(&git.CheckoutOptions{
				Branch: sourceRef,
				Force:  true,
			}); err != nil {
				logger.Errorf("Unable to checkout %s: %v", sourceRef, err)
				return err
			}
		}

		pullOptions := git.PullOptions{
			Force: true,
		}
		if input.Token != "" {
			pullOptions.Auth = &http.BasicAuth{
				Username: "token",
				Password: input.Token,
			}
		}

		if err = w.Pull(&pullOptions); err != nil && err.Error() != "already up-to-date" {
			logger.Tracef("Unable to pull %s: %v", refName, err)
		}
		logger.Tracef("Cloned %s to %s", input.URL, input.Dir)

		if hash.String() != input.Ref && refType == "branch" {
			logger.Tracef("Provided ref is not a sha. Updating branch ref after pull")
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

		logger.Tracef("Checked out %s", input.Ref)
		return nil
	}
}
