package runner

import (
	"context"
	"testing"
)

func TestStepContextExecutor(t *testing.T) {
	platforms := map[string]string{
		"ubuntu-latest": "node:12.6-buster-slim",
	}
	tables := []TestJobFileInfo{
		{"testdata", "invalid-uses-empty", "push", "Expected format {org}/{repo}[/path]@ref", platforms},
		{"testdata", "invalid-uses-noref", "push", "Expected format {org}/{repo}[/path]@ref", platforms},
	}
	ctx := context.Background()
	for _, table := range tables {
		runTestJobFile(ctx, t, table)
	}
}
