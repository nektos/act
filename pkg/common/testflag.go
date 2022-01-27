package common

import (
	"context"
)

type testFlagContextKey string

const testFlagContextKeyVal = testFlagContextKey("test-context")

// TestContext returns whether the context has the test flag set
func TestContext(ctx context.Context) bool {
	val := ctx.Value(testFlagContextKeyVal)
	return val != nil
}

// WithTextContext sets the test flag in the context
func WithTestContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, testFlagContextKeyVal, true)
}
