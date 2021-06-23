package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithDryRun(t *testing.T) {
	for _, b := range []bool{true, false} {
		ctx := WithDryrun(context.TODO(), b)
		assert.Equal(t, b, ctx.Value(dryrunContextKeyVal))
		assert.Equal(t, b, Dryrun(ctx))
	}
	assert.Equal(t, false, Dryrun(context.TODO()))
}
