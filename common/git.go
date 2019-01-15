package common

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/go-ini/ini"
	log "github.com/sirupsen/logrus"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/yaml.v2"
)

var cloneLock sync.Mutex

// FindGitRevision get the current git revision
func FindGitRevision(file string) (shortSha string, sha string, err error) {
	gitDir, err := findGitDirectory(file)
	if err != nil {
		return "", "", err
	}

	head, err := findGitHead(file)
	if err != nil {
		return "", "", err
	}
	// load commitid ref
	refBuf, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", gitDir, head))
	if err != nil {
		return "", "", err
	}
	return string(string(refBuf)[:7]), string(refBuf), nil
}

// FindGitBranch get the current git branch
func FindGitBranch(file string) (string, error) {
	head, err := findGitHead(file)
	if err != nil {
		return "", err
	}

	// get branch name
	branch := strings.TrimPrefix(head, "refs/heads/")
	log.Debugf("Found branch: %s", branch)
	return branch, nil
}

func findGitHead(file string) (string, error) {
	gitDir, err := findGitDirectory(file)
	if err != nil {
		return "", err
	}
	log.Debugf("Loading revision from git directory '%s'", gitDir)

	// load HEAD ref
	headFile, err := os.Open(fmt.Sprintf("%s/HEAD", gitDir))
	if err != nil {
		return "", err
	}
	defer func() {
		headFile.Close()
	}()

	headBuffer := new(bytes.Buffer)
	headBuffer.ReadFrom(bufio.NewReader(headFile))
	head := make(map[string]string)
	yaml.Unmarshal(headBuffer.Bytes(), head)

	log.Debugf("HEAD points to '%s'", head["ref"])

	return head["ref"], nil
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
	codeCommitHTTPRegex := regexp.MustCompile(`^http(s?)://git-codecommit\.(.+)\.amazonaws.com/v1/repos/(.+)$`)
	codeCommitSSHRegex := regexp.MustCompile(`ssh://git-codecommit\.(.+)\.amazonaws.com/v1/repos/(.+)$`)
	httpRegex := regexp.MustCompile("^http(s?)://.*github.com.*/(.+)/(.+).git$")
	sshRegex := regexp.MustCompile("github.com:(.+)/(.+).git$")

	if matches := codeCommitHTTPRegex.FindStringSubmatch(url); matches != nil {
		return "CodeCommit", matches[3], nil
	} else if matches := codeCommitSSHRegex.FindStringSubmatch(url); matches != nil {
		return "CodeCommit", matches[2], nil
	} else if matches := httpRegex.FindStringSubmatch(url); matches != nil {
		return "GitHub", fmt.Sprintf("%s/%s", matches[2], matches[3]), nil
	} else if matches := sshRegex.FindStringSubmatch(url); matches != nil {
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
		dir = path.Dir(absPath)
	}

	gitPath := path.Join(dir, ".git")
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
	URL    *url.URL
	Ref    string
	Dir    string
	Logger *log.Entry
	Dryrun bool
}

// NewGitCloneExecutor creates an executor to clone git repos
func NewGitCloneExecutor(input NewGitCloneExecutorInput) Executor {
	return func() error {
		input.Logger.Infof("git clone '%s'", input.URL.String())
		input.Logger.Debugf("  cloning %s to %s", input.URL.String(), input.Dir)

		if input.Dryrun {
			return nil
		}

		cloneLock.Lock()
		defer cloneLock.Unlock()

		refName := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", input.Ref))

		r, err := git.PlainOpen(input.Dir)
		if err != nil {
			r, err = git.PlainClone(input.Dir, false, &git.CloneOptions{
				URL:           input.URL.String(),
				Progress:      input.Logger.WriterLevel(log.DebugLevel),
				ReferenceName: refName,
			})
			if err != nil {
				return err
			}
		}

		w, err := r.Worktree()
		if err != nil {
			return err
		}

		w.Pull(&git.PullOptions{
			ReferenceName: refName,
		})
		input.Logger.Debugf("Cloned %s to %s", input.URL.String(), input.Dir)

		err = w.Checkout(&git.CheckoutOptions{
			//Branch: plumbing.NewHash(ref),
			Branch: refName,
			//Hash: plumbing.NewHash(input.Ref),
			Force: true,
		})
		if err != nil {
			input.Logger.Errorf("Unable to checkout %s: %v", refName, err)
			return err
		}

		input.Logger.Debugf("Checked out %s", input.Ref)
		return nil
	}
}
