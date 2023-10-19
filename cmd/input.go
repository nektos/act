package cmd

import (
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// Input contains the input for the root command
type Input struct {
	actor                              string
	workdir                            string
	workflowsPath                      string
	autodetectEvent                    bool
	eventPath                          string
	reuseContainers                    bool
	bindWorkdir                        bool
	secrets                            []string
	vars                               []string
	envs                               []string
	inputs                             []string
	platforms                          []string
	dryrun                             bool
	forcePull                          bool
	forceRebuild                       bool
	noOutput                           bool
	envfile                            string
	inputfile                          string
	secretfile                         string
	varfile                            string
	insecureSecrets                    bool
	defaultBranch                      string
	privileged                         bool
	usernsMode                         string
	containerArchitecture              string
	containerDaemonSocket              string
	containerOptions                   string
	noWorkflowRecurse                  bool
	useGitIgnore                       bool
	githubInstance                     string
	containerCapAdd                    []string
	containerCapDrop                   []string
	autoRemove                         bool
	artifactServerPath                 string
	artifactServerAddr                 string
	artifactServerPort                 string
	noCacheServer                      bool
	cacheServerPath                    string
	cacheServerAddr                    string
	cacheServerPort                    uint16
	jsonLogger                         bool
	noSkipCheckout                     bool
	remoteName                         string
	replaceGheActionWithGithubCom      []string
	replaceGheActionTokenWithGithubCom string
	matrix                             []string
	actionCachePath                    string
	logPrefixJobID                     bool
	networkName                        string
}

func (i *Input) resolve(path string) string {
	basedir, err := filepath.Abs(i.workdir)
	if err != nil {
		log.Fatal(err)
	}
	if path == "" {
		return path
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(basedir, path)
	}
	return path
}

// Envfile returns path to .env
func (i *Input) Envfile() string {
	return i.resolve(i.envfile)
}

// Secretfile returns path to secrets
func (i *Input) Secretfile() string {
	return i.resolve(i.secretfile)
}

func (i *Input) Varfile() string {
	return i.resolve(i.varfile)
}

// Workdir returns path to workdir
func (i *Input) Workdir() string {
	return i.resolve(".")
}

// WorkflowsPath returns path to workflow file(s)
func (i *Input) WorkflowsPath() string {
	return i.resolve(i.workflowsPath)
}

// EventPath returns the path to events file
func (i *Input) EventPath() string {
	return i.resolve(i.eventPath)
}

// Inputfile returns the path to the input file
func (i *Input) Inputfile() string {
	return i.resolve(i.inputfile)
}
