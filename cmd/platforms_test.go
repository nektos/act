package cmd

import (
	"testing"

	"github.com/nektos/act/pkg/runner"
	"github.com/stretchr/testify/assert"
)

func TestNewPlatformOptions(t *testing.T) {
	t.Run("replace", func(t *testing.T) {
		input := &Input{
			platformOptions: []string{"big-cpu-runner=--cpus 16"},
		}

		assert.Equal(t, map[string]runner.PlatformOptions{
			"big-cpu-runner": {
				Options: "--cpus 16",
			},
		}, input.newPlatformOptions())
	})

	t.Run("append", func(t *testing.T) {
		input := &Input{
			platformOptions: []string{"gpu-runner+=--gpus all"},
		}

		assert.Equal(t, map[string]runner.PlatformOptions{
			"gpu-runner": {
				Options: "--gpus all",
				Append:  true,
			},
		}, input.newPlatformOptions())
	})
}
