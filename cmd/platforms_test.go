package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPlatforms(t *testing.T) {
	tests := []struct {
		name      string
		platforms []string
		expected  map[string]string
	}{
		{
			name:      "default platforms",
			platforms: []string{},
			expected: map[string]string{
				"ubuntu-latest": "node:16-buster-slim",
				"ubuntu-22.04":  "node:16-bullseye-slim",
				"ubuntu-20.04":  "node:16-buster-slim",
				"ubuntu-18.04":  "node:16-buster-slim",
			},
		},
		{
			name:      "simple platform override",
			platforms: []string{"ubuntu-latest=custom:image"},
			expected: map[string]string{
				"ubuntu-latest": "custom:image",
				"ubuntu-22.04":  "node:16-bullseye-slim",
				"ubuntu-20.04":  "node:16-buster-slim",
				"ubuntu-18.04":  "node:16-buster-slim",
			},
		},
		{
			name:      "platform with multiple equals signs",
			platforms: []string{"runner=fast-disk-network=catthehacker/ubuntu:act-22.04"},
			expected: map[string]string{
				"ubuntu-latest":            "node:16-buster-slim",
				"ubuntu-22.04":             "node:16-bullseye-slim",
				"ubuntu-20.04":             "node:16-buster-slim",
				"ubuntu-18.04":             "node:16-buster-slim",
				"runner=fast-disk-network": "catthehacker/ubuntu:act-22.04",
			},
		},
		{
			name: "multiple platform overrides",
			platforms: []string{
				"ubuntu-latest=custom:image",
				"runner=fast-disk-network=catthehacker/ubuntu:act-22.04",
			},
			expected: map[string]string{
				"ubuntu-latest":            "custom:image",
				"ubuntu-22.04":             "node:16-bullseye-slim",
				"ubuntu-20.04":             "node:16-buster-slim",
				"ubuntu-18.04":             "node:16-buster-slim",
				"runner=fast-disk-network": "catthehacker/ubuntu:act-22.04",
			},
		},
		{
			name:      "case insensitive platform key",
			platforms: []string{"UBUNTU-LATEST=custom:image"},
			expected: map[string]string{
				"ubuntu-latest": "custom:image",
				"ubuntu-22.04":  "node:16-bullseye-slim",
				"ubuntu-20.04":  "node:16-buster-slim",
				"ubuntu-18.04":  "node:16-buster-slim",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &Input{
				platforms: tt.platforms,
			}
			result := input.newPlatforms()
			assert.Equal(t, tt.expected, result)
		})
	}
}
