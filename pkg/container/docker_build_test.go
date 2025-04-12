package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactProxyURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://foo:bar@example.com:1234",
			expected: "https://foo:xxxxx@example.com:1234",
		},
		{
			input:    "https://foo:bar@example.com",
			expected: "https://foo:xxxxx@example.com",
		},
		{
			input:    "https://foo:bar%40@example.com:1234",
			expected: "https://foo:xxxxx@example.com:1234",
		},
		{
			input:    "https://example.com:1234",
			expected: "https://example.com:1234",
		},
		{
			input:    "https://broken@example.com",
			expected: "https://broken@example.com",
		},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			redacted := redactProxyURL(tc.input)
			assert.Equal(t, redacted, tc.expected)
		})
	}
}
