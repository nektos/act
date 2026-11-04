//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunnerContextUsesContainerArchitecture(t *testing.T) {
	environment := NewContainer(&NewContainerInput{Platform: "linux/amd64"})

	assert.Equal(t, "X64", environment.GetRunnerContext(context.Background())["arch"])
}
