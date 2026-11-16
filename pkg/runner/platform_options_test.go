package runner

import (
	"context"
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestContainerOptions(t *testing.T) {
	platformOptions := map[string]PlatformOptions{
		"big-cpu-runner": {
			Options: "--cpus 16",
		},
		"gpu-runner": {
			Options: "--gpus all",
			Append:  true,
		},
	}

	tests := []struct {
		name     string
		job      string
		expected string
	}{
		{"platform replacement", `runs-on: [self-hosted, big-cpu-runner]`, "--cpus 16"},
		{"platform append", `runs-on: [self-hosted, gpu-runner]`, "--cpus 4 --gpus all"},
		{"global fallback", `runs-on: [self-hosted, cpu-runner]`, "--cpus 4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := createIfTestRunContext(map[string]*model.Job{
				"job1": createJob(t, tt.job, ""),
			})
			rc.Config.ContainerOptions = "--cpus 4"
			rc.Config.PlatformOptions = platformOptions

			assert.Equal(t, tt.expected, rc.containerOptions(context.Background()))
		})
	}
}
