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
	assert.NoError(t, err, "Expected no error from GetSocketAndHost")

	// Assert that ret.Socket and ret.Host are as expected
	assert.Equal(t, socketURI, ret.Socket, "Expected ret.Socket to match socketURI")
	// assert.Equal(t, socketURI, ret.Host, "Expected ret.Host to match socketURI")

	// Expand environment variables in CommonSocketLocations
	expandedLocations := make([]string, len(CommonSocketLocations))
	for i, loc := range CommonSocketLocations {
		expandedLocations[i] = os.ExpandEnv(loc)
	}

	// Assert that ret is in the list of expanded common locations
	assert.Contains(t, expandedLocations, ret.Socket, "Expected ret.Socket to be in the list of common locations")
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
