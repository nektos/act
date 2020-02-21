package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/go-ini/ini"
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

	log.Debugf("Found revision: %s", refBuf)
	return string(refBuf[:7]), strings.TrimSpace(string(refBuf)), nil
}

// FindGitRef get the current git ref
func FindGitRef(file string) (string, error) {
	gitDir, err := findGitDirectory(file)
	if err != nil {
		return "", err
	}
	log.Debugf("Loading revision from git directory '%s'", gitDir)

	_, ref, err := FindGitRevision(file)
	if err != nil {
		return "", err
	}

	log.Debugf("HEAD points to '%s'", ref)

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
			return nil
		}
		if name != "" {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		bts, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		var pointsTo = strings.TrimSpace(string(bts))
		if head == pointsTo {
			name = strings.TrimPrefix(strings.Replace(path, root, "", 1), "/")
			log.Debugf("HEAD matches %s", name)
		}
		return nil
	})
	return name, err
}

// FindGithubRepo get the repo
func FindGithubRepo(file string) (string, error) {
	url, err := findGitRemoteURL(file)
	if err != nil {
		return "", err
	}
	_, slug, err := findGitSlug(url)
	return slug, err
}

func findGitRemoteURL(file string) (string, error) {
	gitDir, err := findGitDirectory(file)
	if err != nil {
		return "", err
	}
	log.Debugf("Loading slug from git directory '%s'", gitDir)

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

func findGitDirectory(fromFile string) (string, error) {
	absPath, err := filepath.Abs(fromFile)
	if err != nil {
		return "", err
	}

	log.Debugf("Searching for git directory in %s", absPath)
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

		if Dryrun(ctx) {
			return nil
		}

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
			logger.Errorf("Unable to pull %s: %v", refName, err)
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
			logger.Errorf("Unable to checkout %s: %v", refName, err)
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
