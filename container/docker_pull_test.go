package container

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestCleanImage(t *testing.T) {
	tables := []struct {
		imageIn  string
		imageOut string
	}{
		{"myhost.com/foo/bar", "myhost.com/foo/bar"},
		{"ubuntu", "docker.io/library/ubuntu"},
		{"ubuntu:18.04", "docker.io/library/ubuntu:18.04"},
		{"cibuilds/hugo:0.53", "docker.io/cibuilds/hugo:0.53"},
	}

	for _, table := range tables {
		imageOut := cleanImage(table.imageIn)
		assert.Equal(t, table.imageOut, imageOut)
	}
}
