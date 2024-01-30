package container

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/creack/pty"
)

func getSysProcAttr(_ string, tty bool) *syscall.SysProcAttr {
	if tty {
		return &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
		}
	}
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func openPty() (*os.File, *os.File, error) {
	return pty.Open()
}

// Consolidate common socket stuff
var commonSocketPaths = []string{
	"/var/run/docker.sock",
	"/run/podman/podman.sock",
	"$HOME/.colima/docker.sock",
	"$XDG_RUNTIME_DIR/docker.sock",
	"$XDG_RUNTIME_DIR/podman/podman.sock",
	`\\.\pipe\docker_engine`,
	"$HOME/.docker/run/docker.sock",
}

func GetCommonSocketPaths() ([]string, bool) {
	return commonSocketPaths, true
}

// returns socket path or false if not found any
func SocketLocation() (string, bool) {
	if dockerHost, exists := os.LookupEnv("DOCKER_HOST"); exists {
		return dockerHost, true
	}

	for _, p := range commonSocketPaths {
		if _, err := os.Lstat(os.ExpandEnv(p)); err == nil {
			if strings.HasPrefix(p, `\\.\`) {
				return "npipe://" + filepath.ToSlash(os.ExpandEnv(p)), true
			}
			return "unix://" + filepath.ToSlash(os.ExpandEnv(p)), true
		}
	}

	return "", false
}

func GetDockerDaemonSocketMountPath(daemonPath string) string {
	if protoIndex := strings.Index(daemonPath, "://"); protoIndex != -1 {
		scheme := daemonPath[:protoIndex]
		if strings.EqualFold(scheme, "npipe") {
			// linux container mount on windows, use the default socket path of the VM / wsl2
			return "/var/run/docker.sock"
		} else if strings.EqualFold(scheme, "unix") {
			return daemonPath[protoIndex+3:]
		} else if strings.IndexFunc(scheme, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
		}) == -1 {
			// unknown protocol use default
			socket, _ := SocketLocation()
			// Strip protocol prefix
			return strings.TrimPrefix(strings.TrimPrefix(socket, "unix://"), "npipe://")
		}
	}
	return daemonPath
}
