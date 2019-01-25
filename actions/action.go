package actions

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nektos/act/common"
	log "github.com/sirupsen/logrus"
)

// imageURL is the directory where a `Dockerfile` should exist
func parseImageLocal(workingDir string, contextDir string) (contextDirOut string, tag string, ok bool) {
	if !strings.HasPrefix(contextDir, "./") {
		return "", "", false
	}
	contextDir = filepath.Join(workingDir, contextDir)
	if _, err := os.Stat(filepath.Join(contextDir, "Dockerfile")); os.IsNotExist(err) {
		log.Debugf("Ignoring missing Dockerfile '%s/Dockerfile'", contextDir)
		return "", "", false
	}

	sha, _, err := common.FindGitRevision(contextDir)
	if err != nil {
		log.Warnf("Unable to determine git revision: %v", err)
		sha = "latest"
	}
	return contextDir, fmt.Sprintf("%s:%s", filepath.Base(contextDir), sha), true
}

// imageURL is the URL for a docker repo
func parseImageReference(image string) (ref string, ok bool) {
	imageURL, err := url.Parse(image)
	if err != nil {
		log.Debugf("Unable to parse image as url: %v", err)
		return "", false
	}
	if imageURL.Scheme != "docker" {
		log.Debugf("Ignoring non-docker ref '%s'", imageURL.String())
		return "", false
	}

	return fmt.Sprintf("%s%s", imageURL.Host, imageURL.Path), true
}

// imageURL is the directory where a `Dockerfile` should exist
func parseImageGithub(image string) (cloneURL *url.URL, ref string, path string, ok bool) {
	re := regexp.MustCompile("^([^/@]+)/([^/@]+)(/([^@]*))?(@(.*))?$")
	matches := re.FindStringSubmatch(image)

	if matches == nil {
		return nil, "", "", false
	}

	cloneURL, err := url.Parse(fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2]))
	if err != nil {
		log.Debugf("Unable to parse as URL: %v", err)
		return nil, "", "", false
	}

	resp, err := http.Head(cloneURL.String())
	if resp.StatusCode >= 400 || err != nil {
		log.Debugf("Unable to HEAD URL %s status=%v err=%v", cloneURL.String(), resp.StatusCode, err)
		return nil, "", "", false
	}

	ref = matches[6]
	if ref == "" {
		ref = "master"
	}

	path = matches[4]
	if path == "" {
		path = "."
	}

	return cloneURL, ref, path, true
}
