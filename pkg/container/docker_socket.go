package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

var CommonSocketLocations = []string{
	"/var/run/docker.sock",
	"/run/podman/podman.sock",
	"$HOME/.colima/docker.sock",
	"$XDG_RUNTIME_DIR/docker.sock",
	"$XDG_RUNTIME_DIR/podman/podman.sock",
	`\\.\pipe\docker_engine`,
	"$HOME/.docker/run/docker.sock",
}

// returns socket URI or false if not found any
func socketLocation() (string, bool) {
	if dockerHost, exists := os.LookupEnv("DOCKER_HOST"); exists {
		return dockerHost, true
	}

	for _, p := range CommonSocketLocations {
		if _, err := os.Lstat(os.ExpandEnv(p)); err == nil {
			if strings.HasPrefix(p, `\\.\`) {
				return "npipe://" + filepath.ToSlash(os.ExpandEnv(p)), true
			}
			return "unix://" + filepath.ToSlash(os.ExpandEnv(p)), true
		}
	}

	return "", false
}

// This function, `isDockerHostURI`, takes a string argument `daemonPath`. It checks if the
// `daemonPath` is a valid Docker host URI. It does this by checking if the scheme of the URI (the
// part before "://") contains only alphabetic characters. If it does, the function returns true,
// indicating that the `daemonPath` is a Docker host URI. If it doesn't, or if the "://" delimiter
// is not found in the `daemonPath`, the function returns false.
func isDockerHostURI(daemonPath string) bool {
	if protoIndex := strings.Index(daemonPath, "://"); protoIndex != -1 {
		scheme := daemonPath[:protoIndex]
		if strings.IndexFunc(scheme, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
		}) == -1 {
			return true
		}
	}
	return false
}

type SocketAndHost struct {
	Socket string
	Host   string
}

func GetSocketAndHost(containerSocket string) (SocketAndHost, error) {
	log.Debugf("Handling container host and socket")

	// Prefer DOCKER_HOST, don't override it
	dockerHost, hasDockerHost := socketLocation()
	socketHost := SocketAndHost{Socket: containerSocket, Host: dockerHost}

	// ** socketHost.Socket cases **
	// Case 1: User does _not_ want to mount a daemon socket (passes a dash)
	// Case 2: User passes a filepath to the socket; is that even valid?
	// Case 3: User passes a valid socket; do nothing
	// Case 4: User omitted the flag; set a sane default

	// ** DOCKER_HOST cases **
	// Case A: DOCKER_HOST is set; use it, i.e. do nothing
	// Case B: DOCKER_HOST is empty; use sane defaults

	// Set host for sanity's sake, when the socket isn't useful
	if !hasDockerHost && (socketHost.Socket == "-" || !isDockerHostURI(socketHost.Socket) || socketHost.Socket == "") {
		// Cases: 1B, 2B, 4B
		socket, found := socketLocation()
		socketHost.Host = socket
		hasDockerHost = found
	}

	// A - (dash) in socketHost.Socket means don't mount, preserve this value
	// otherwise if socketHost.Socket is a filepath don't use it as socket
	// Exit early if we're in an invalid state (e.g. when no DOCKER_HOST and user supplied "-", a dash or omitted)
	if !hasDockerHost && socketHost.Socket != "" && !isDockerHostURI(socketHost.Socket) {
		// Cases: 1B, 2B
		// Should we early-exit here, since there is no host nor socket to talk to?
		return SocketAndHost{}, fmt.Errorf("DOCKER_HOST was not set, couldn't be found in the usual locations, and the container daemon socket ('%s') is invalid", socketHost.Socket)
	}

	// Default to DOCKER_HOST if set
	if socketHost.Socket == "" && hasDockerHost {
		// Cases: 4A
		log.Debugf("Defaulting container socket to DOCKER_HOST")
		socketHost.Socket = socketHost.Host
	}
	// Set sane default socket location if user omitted it
	if socketHost.Socket == "" {
		// Cases: 4B
		socket, _ := socketLocation()
		// socket is empty if it isn't found, so assignment here is at worst a no-op
		log.Debugf("Defaulting container socket to default '%s'", socket)
		socketHost.Socket = socket
	}

	// Exit if both the DOCKER_HOST and socket are fulfilled
	if hasDockerHost {
		// Cases: 1A, 2A, 3A, 4A
		if !isDockerHostURI(socketHost.Socket) {
			// Cases: 1A, 2A
			log.Debugf("DOCKER_HOST is set, but socket is invalid '%s'", socketHost.Socket)
		}
		return socketHost, nil
	}

	// Set a sane DOCKER_HOST default if we can
	if isDockerHostURI(socketHost.Socket) {
		// Cases: 3B
		log.Debugf("Setting DOCKER_HOST to container socket '%s'", socketHost.Socket)
		socketHost.Host = socketHost.Socket
		// Both DOCKER_HOST and container socket are valid; short-circuit exit
		return socketHost, nil
	}

	// Here there is no DOCKER_HOST _and_ the supplied container socket is not a valid URI (either invalid or a file path)
	// Cases: 2B <- but is already handled at the top
	// I.e. this path should never be taken
	return SocketAndHost{}, fmt.Errorf("no DOCKER_HOST and an invalid container socket '%s'", socketHost.Socket)
}
