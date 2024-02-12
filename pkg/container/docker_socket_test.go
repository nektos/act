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

var originalCommonSocketLocations = CommonSocketLocations

func TestGetSocketAndHostWithSocket(t *testing.T) {
	// Arrange
	CommonSocketLocations = originalCommonSocketLocations
	dockerHost := "unix:///my/docker/host.sock"
	socketURI := "/path/to/my.socket"
	os.Setenv("DOCKER_HOST", dockerHost)

	// Act
	ret, err := GetSocketAndHost(socketURI)

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, SocketAndHost{socketURI, dockerHost}, ret)
}

func TestGetSocketAndHostNoSocket(t *testing.T) {
	// Arrange
	dockerHost := "unix:///my/docker/host.sock"
	os.Setenv("DOCKER_HOST", dockerHost)

	// Act
	ret, err := GetSocketAndHost("")

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, SocketAndHost{dockerHost, dockerHost}, ret)
}

func TestGetSocketAndHostOnlySocket(t *testing.T) {
	// Arrange
	socketURI := "/path/to/my.socket"
	os.Unsetenv("DOCKER_HOST")
	CommonSocketLocations = originalCommonSocketLocations
	defaultSocket, defaultSocketFound := socketLocation()

	// Act
	ret, err := GetSocketAndHost(socketURI)

	// Assert
	assert.NoError(t, err, "Expected no error from GetSocketAndHost")
	assert.Equal(t, true, defaultSocketFound, "Expected to find default socket")
	assert.Equal(t, socketURI, ret.Socket, "Expected socket to match common location")
	assert.Equal(t, defaultSocket, ret.Host, "Expected ret.Host to match default socket location")
}

func TestGetSocketAndHostDontMount(t *testing.T) {
	// Arrange
	CommonSocketLocations = originalCommonSocketLocations
	dockerHost := "unix:///my/docker/host.sock"
	os.Setenv("DOCKER_HOST", dockerHost)

	// Act
	ret, err := GetSocketAndHost("-")

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, SocketAndHost{"-", dockerHost}, ret)
}

func TestGetSocketAndHostNoHostNoSocket(t *testing.T) {
	// Arrange
	CommonSocketLocations = originalCommonSocketLocations
	os.Unsetenv("DOCKER_HOST")
	defaultSocket, found := socketLocation()

	// Act
	ret, err := GetSocketAndHost("")

	// Assert
	assert.Equal(t, true, found, "Expected a default socket to be found")
	assert.Nil(t, err, "Expected no error from GetSocketAndHost")
	assert.Equal(t, SocketAndHost{defaultSocket, defaultSocket}, ret, "Expected to match default socket location")
}

// Catch
// > Your code breaks setting DOCKER_HOST if shouldMount is false.
// > This happens if neither DOCKER_HOST nor --container-daemon-socket has a value, but socketLocation() returns a URI
func TestGetSocketAndHostNoHostNoSocketDefaultLocation(t *testing.T) {
	// Arrange
	mySocketFile, tmpErr := os.CreateTemp("", "act-*.sock")
	mySocket := mySocketFile.Name()
	unixSocket := "unix://" + mySocket
	defer os.RemoveAll(mySocket)
	assert.NoError(t, tmpErr)
	os.Unsetenv("DOCKER_HOST")

	CommonSocketLocations = []string{mySocket}
	defaultSocket, found := socketLocation()

	// Act
	ret, err := GetSocketAndHost("")

	// Assert
	assert.Equal(t, unixSocket, defaultSocket, "Expected default socket to match common socket location")
	assert.Equal(t, true, found, "Expected default socket to be found")
	assert.Nil(t, err, "Expected no error from GetSocketAndHost")
	assert.Equal(t, SocketAndHost{unixSocket, unixSocket}, ret, "Expected to match default socket location")
}

func TestGetSocketAndHostNoHostInvalidSocket(t *testing.T) {
	// Arrange
	os.Unsetenv("DOCKER_HOST")
	mySocket := "/my/socket/path.sock"
	CommonSocketLocations = []string{"/unusual", "/socket", "/location"}
	defaultSocket, found := socketLocation()

	// Act
	ret, err := GetSocketAndHost(mySocket)

	// Assert
	assert.Equal(t, false, found, "Expected no default socket to be found")
	assert.Equal(t, "", defaultSocket, "Expected no default socket to be found")
	assert.Equal(t, SocketAndHost{}, ret, "Expected to match default socket location")
	assert.Error(t, err, "Expected an error in invalid state")
}

func TestGetSocketAndHostOnlySocketValidButUnusualLocation(t *testing.T) {
	// Arrange
	socketURI := "unix:///path/to/my.socket"
	CommonSocketLocations = []string{"/unusual", "/location"}
	os.Unsetenv("DOCKER_HOST")
	defaultSocket, found := socketLocation()

	// Act
	ret, err := GetSocketAndHost(socketURI)

	// Assert
	// Default socket locations
	assert.Equal(t, "", defaultSocket, "Expect default socket location to be empty")
	assert.Equal(t, false, found, "Expected no default socket to be found")
	// Sane default
	assert.Nil(t, err, "Expect no error from GetSocketAndHost")
	assert.Equal(t, socketURI, ret.Host, "Expect host to default to unusual socket")
}
