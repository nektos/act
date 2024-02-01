package container

import (
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestGetSocketAndHostWithSocket(t *testing.T) {
	// Arrange
	dockerHost := "unix:///my/docker/host.sock"
	socketURI := "/path/to/my.socket"
	os.Setenv("DOCKER_HOST", dockerHost)

	// Act
	ret, err := GetSocketAndHost(socketURI)

	// Assert
	assert.Equal(t, err, nil)
	assert.Equal(t, SocketAndHost{socketURI, dockerHost}, ret)
}

func TestGetSocketAndHostNoSocket(t *testing.T) {
	// Arrange
	dockerHost := "unix:///my/docker/host.sock"
	os.Setenv("DOCKER_HOST", dockerHost)

	// Act
	ret, err := GetSocketAndHost("")

	// Assert
	assert.Equal(t, err, nil)
	assert.Equal(t, SocketAndHost{dockerHost, dockerHost}, ret)
}

func TestGetSocketAndHostOnlySocket(t *testing.T) {
	// Arrange
	socketURI := "/path/to/my.socket"
	os.Unsetenv("DOCKER_HOST")

	// Act
	ret, err := GetSocketAndHost(socketURI)

	// Assert
	assert.Equal(t, err, nil)
	// assert.Equal(t, SocketAndHost{socketURI, socketURI}, ret)
	assert.Equal(t, socketURI, ret)
}

func TestGetSocketAndHostDontMount(t *testing.T) {
	// Arrange
	dockerHost := "unix:///my/docker/host.sock"
	os.Setenv("DOCKER_HOST", dockerHost)

	// Act
	ret, err := GetSocketAndHost("-")

	// Assert
	assert.Equal(t, err, nil)
	assert.Equal(t, SocketAndHost{"-", dockerHost}, ret)
}
