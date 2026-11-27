package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPlatformsDefaults(t *testing.T) {
	i := &Input{}
	platforms := i.newPlatforms()

	// ubuntu-latest must be present and resolve to a default image
	latest, ok := platforms["ubuntu-latest"]
	assert.True(t, ok, "ubuntu-latest should be a default platform")
	assert.NotEmpty(t, latest)

	// ubuntu-slim should fall back to the same image as ubuntu-latest
	// (GitHub's 1-vCPU runner uses the same base distro, see issue #6003)
	slim, ok := platforms["ubuntu-slim"]
	assert.True(t, ok, "ubuntu-slim should be a default platform")
	assert.Equal(t, latest, slim, "ubuntu-slim should default to the same image as ubuntu-latest")
}

func TestNewPlatformsUserOverride(t *testing.T) {
	i := &Input{platforms: []string{"ubuntu-slim=catthehacker/ubuntu:act-latest"}}
	platforms := i.newPlatforms()

	assert.Equal(t, "catthehacker/ubuntu:act-latest", platforms["ubuntu-slim"],
		"user-supplied -P value should override the default")
}
