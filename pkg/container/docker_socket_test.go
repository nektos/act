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
	os.Setenv("DOCKER_HOST", "unix:///my/docker/host.sock")
	ret, err := GetSocketAndHost("/path/to/my.socket")
	assert.NotErrorIs(t, err, nil)
	assert.Equal(t, SocketAndHost{"/path/to/my.socket", "unix:///my/docker/host.sock"}, ret)
}

func TestGetSocketAndHostNoSocket(t *testing.T) {
	os.Setenv("DOCKER_HOST", "unix:///my/docker/host.sock")
	ret, err := GetSocketAndHost("")
	assert.NotErrorIs(t, err, nil)
	assert.Equal(t, SocketAndHost{"/var/run/docker.sock", "unix:///my/docker/host.sock"}, ret)
}

func TestGetSocketAndHostOnlySocket(t *testing.T) {
	os.Setenv("DOCKER_HOST", "unix:///my/docker/host.sock")
	os.Unsetenv("DOCKER_HOST")
	ret, err := GetSocketAndHost("/path/to/my.socket")
	assert.NotErrorIs(t, err, nil)
	assert.Equal(t, SocketAndHost{"/path/to/my.socket", "unix:///my/docker/host.sock"}, ret)
}

func TestGetSocketAndHostDontMount(t *testing.T) {
	os.Setenv("DOCKER_HOST", "unix:///my/docker/host.sock")
	ret, err := GetSocketAndHost("-")
	assert.NotErrorIs(t, err, nil)
	assert.Equal(t, SocketAndHost{"-", "unix:///my/docker/host.sock"}, ret)
}

func TestGetSocketAndHost(t *testing.T) {
	os.Setenv("DOCKER_HOST", "unix:///my/docker/host.sock")
	ret, err := GetSocketAndHost("-")
	assert.NotErrorIs(t, err, nil)
	assert.Equal(t, SocketAndHost{"-", "unix:///my/docker/host.sock"}, ret)
}
