package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeEnvFromImage(t *testing.T) {
	inputEnv := []string{
		"PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/sbin",
		"GOPATH=/root/go",
		"GOOS=linux",
	}
	imageEnv := []string{
		"PATH=/root/go/bin",
		"GOPATH=/tmp",
		"GOARCH=amd64",
	}

	merged := mergeEnvFromImage(inputEnv, imageEnv)

	assert.Equal(t, []string{
		"PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/sbin:/root/go/bin",
		"GOPATH=/root/go",
		"GOOS=linux",
		"GOARCH=amd64",
	}, merged)
}
