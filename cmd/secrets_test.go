package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSecretsPreservesEmptyEnvironmentValue(t *testing.T) {
	t.Setenv("ACT_TEST_EMPTY", "")
	t.Setenv("ACT_TEST_VALUE", "value")
	t.Setenv("ACT_TEST_EXPLICIT", "environment fallback")

	values := newSecrets([]string{"ACT_TEST_EMPTY", "ACT_TEST_VALUE", "ACT_TEST_EXPLICIT="})
	assert.Equal(t, map[string]string{
		"ACT_TEST_EMPTY":    "",
		"ACT_TEST_EXPLICIT": "",
		"ACT_TEST_VALUE":    "value",
	}, values.AsMap())
}
