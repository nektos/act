package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimplePlatform(t *testing.T) {
	inpt := &Input{
		platforms: []string{"ubuntu-latest=node:16-buster-slim"},
	}
	resp := inpt.newPlatforms()
	assert.Contains(t, resp, "ubuntu-latest")
}

func TestMultiEqPlatform(t *testing.T) {
	inpt := &Input{
		platforms: []string{"runs-on=1/foo=bar=node:18"},
	}
	resp := inpt.newPlatforms()
	assert.Contains(t, resp, "runs-on=1/foo=bar")
	assert.Equal(t, resp["runs-on=1/foo=bar"], "node:18")
}

func TestInvalidPlatformMissing(t *testing.T) {
	inpt := &Input{
		platforms: []string{"=node:18"},
	}
	resp := inpt.newPlatforms()
	assert.Len(t, resp, 4)
}

func TestInvalidPlatformImageOnly(t *testing.T) {
	inpt := &Input{
		platforms: []string{"node:18"},
	}
	resp := inpt.newPlatforms()
	assert.Len(t, resp, 4)
}
