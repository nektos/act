package runner

import (
	"context"
	"testing"

	"github.com/nektos/act/pkg/common"
)

func TestStepContextExecutor(t *testing.T) {
	platforms := map[string]string{
		"ubuntu-latest": "node:12.6-buster-slim",
	}
	tables := []TestJobFileInfo{
		{"testdata", "uses-github-empty", "push", "Expected format {org}/{repo}[/path]@ref", platforms},
		{"testdata", "uses-github-noref", "push", "Expected format {org}/{repo}[/path]@ref", platforms},
		{"testdata", "uses-github-root", "push", "", platforms},
		{"testdata", "uses-github-path", "push", "", platforms},
	}
	// These tests are sufficient to only check syntax.
	ctx := common.WithDryrun(context.Background(), true)
	for _, table := range tables {
		runTestJobFile(ctx, t, table)
	}
}
