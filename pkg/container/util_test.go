package container

import (
	"testing"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestGetSocketAndHostWithSocket(t *testing.T) {
	ret, err := GetSocketAndHost("/path/to/my.socket")
	assert.NotErrorIs(t, err, nil)
	assert.Equal(t, SocketAndHost{"/path/to/my.socket", "/var/run/docker.sock"}, ret)
}

func TestGetSocketAndHostNoSocket(t *testing.T) {
	ret, err := GetSocketAndHost("")
	assert.NotErrorIs(t, err, nil)
	assert.Equal(t, SocketAndHost{"/var/run/docker.sock", "/var/run/docker.sock"}, ret)
}

func TestGetSocketAndHostOnlySocket(t *testing.T) {
	ret, err := GetSocketAndHost("/path/to/my.socket")
	assert.NotErrorIs(t, err, nil)
	assert.Equal(t, SocketAndHost{"/path/to/my.socket", "/path/to/my.socket"}, ret)
}

func TestGetSocketAndHostDontMount(t *testing.T) {
	ret, err := GetSocketAndHost("-")
	assert.NotErrorIs(t, err, nil)
	assert.Equal(t, SocketAndHost{"-", "/var/run/docker.sock"}, ret)
}
