package config

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Config contains the config for a new runner
type Config struct {
	Actor                 string            // the user that triggered the event
	Workdir               string            // path to working directory
	BindWorkdir           bool              // bind the workdir to the job container
	EventName             string            // name of event to run
	EventPath             string            // path to JSON file to use for event.json in containers
	DefaultBranch         string            // name of the main branch for this repository
	ReuseContainers       bool              // reuse containers to maintain state
	ForcePull             bool              // force pulling of the image, even if already present
	ForceRebuild          bool              // force rebuilding local docker image action
	LogOutput             bool              // log the output from docker run
	JSONLogger            bool              // use json or text logger
	Env                   map[string]string // env for containers
	Secrets               map[string]string // list of secrets
	InsecureSecrets       bool              // switch hiding output when printing to terminal
	Platforms             map[string]string // list of platforms
	Privileged            bool              // use privileged mode
	UsernsMode            string            // user namespace to use
	ContainerArchitecture string            // Desired OS/architecture platform for running containers
	ContainerDaemonSocket string            // Path to Docker daemon socket
	UseGitIgnore          bool              // controls if paths in .gitignore should not be copied into container, default true
	GitHubInstance        string            // GitHub instance to use, default "github.com"
	ContainerCapAdd       []string          // list of kernel capabilities to add to the containers
	ContainerCapDrop      []string          // list of kernel capabilities to remove from the containers
	AutoRemove            bool              // controls if the container is automatically removed upon workflow completion
	ArtifactServerPath    string            // the path where the artifact server stores uploads
	ArtifactServerPort    string            // the port the artifact server binds to
	CompositeRestrictions any               // describes which features are available in composite actions
	NoSkipCheckout        bool              // do not skip actions/checkout
}

// ContainerPath resolves the equivalent host path inside the container
// This is required for windows and WSL 2 to translate things like C:\Users\Myproject to /mnt/users/Myproject
// For use in docker volumes and binds
func (config *Config) containerPath(path string) string {
	if runtime.GOOS == "windows" && strings.Contains(path, "/") {
		log.Error("You cannot specify linux style local paths (/mnt/etc) on Windows as it does not understand them.")
		return ""
	}

	abspath, err := filepath.Abs(path)
	if err != nil {
		log.Error(err)
		return ""
	}

	// Test if the path is a windows path
	windowsPathRegex := regexp.MustCompile(`^([a-zA-Z]):\\(.+)$`)
	windowsPathComponents := windowsPathRegex.FindStringSubmatch(abspath)

	// Return as-is if no match
	if windowsPathComponents == nil {
		return abspath
	}

	// Convert to WSL2-compatible path if it is a windows path
	// NOTE: Cannot use filepath because it will use the wrong path separators assuming we want the path to be windows
	// based if running on Windows, and because we are feeding this to Docker, GoLang auto-path-translate doesn't work.
	driveLetter := strings.ToLower(windowsPathComponents[1])
	translatedPath := strings.ReplaceAll(windowsPathComponents[2], `\`, `/`)
	// Should make something like /mnt/c/Users/person/My Folder/MyActProject
	result := strings.Join([]string{"/mnt", driveLetter, translatedPath}, `/`)
	return result
}

// ContainerWorkdir resolves the equivalent host path inside the container
// This is required for windows and WSL 2 to translate things like C:\Users\Myproject to /mnt/users/Myproject
func (config *Config) ContainerWorkdir() string {
	return config.containerPath(config.Workdir)
}
