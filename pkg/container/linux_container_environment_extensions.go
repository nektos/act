package container

import (
	"context"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

type LinuxContainerEnvironmentExtensions struct {
}

// ToContainerPath resolves the equivalent host path inside the container
// This is required for windows and WSL 2 to translate things like C:\Users\Myproject to /mnt/users/Myproject
// For use in docker volumes and binds
func (*LinuxContainerEnvironmentExtensions) ToContainerPath(path string) string {
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

func (*LinuxContainerEnvironmentExtensions) GetActPath() string {
	return "/var/run/act"
}

func (*LinuxContainerEnvironmentExtensions) GetPathVariableName() string {
	return "PATH"
}

func (*LinuxContainerEnvironmentExtensions) DefaultPathVariable() string {
	return "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
}

func (*LinuxContainerEnvironmentExtensions) JoinPathVariable(paths ...string) string {
	return strings.Join(paths, ":")
}

func (*LinuxContainerEnvironmentExtensions) GetRunnerContext(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"os":         "Linux",
		"arch":       RunnerArch(ctx),
		"temp":       "/tmp",
		"tool_cache": "/opt/hostedtoolcache",
	}
}
