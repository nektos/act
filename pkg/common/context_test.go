package common

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGracefulJobCancellationViaSigint(t *testing.T) {
	ctx, cancel, channel := createGracefulJobCancellationContext()
	defer cancel()
	assert.NotNil(t, ctx)
	assert.NotNil(t, cancel)
	assert.NotNil(t, channel)
	cancelCtx := JobCancelContext(ctx)
	assert.NotNil(t, cancelCtx)
	assert.NoError(t, ctx.Err())
	assert.NoError(t, cancelCtx.Err())
	channel <- os.Interrupt
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("context not canceled")
	case <-cancelCtx.Done():
	case <-ctx.Done():
	}
	if assert.Error(t, cancelCtx.Err(), "context canceled") {
		assert.Equal(t, context.Canceled, cancelCtx.Err())
	}
	assert.NoError(t, ctx.Err())
	channel <- os.Interrupt
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("context not canceled")
	case <-ctx.Done():
	}
	if assert.Error(t, ctx.Err(), "context canceled") {
		assert.Equal(t, context.Canceled, ctx.Err())
	}
}

func TestForceCancellationViaSigterm(t *testing.T) {
	ctx, cancel, channel := createGracefulJobCancellationContext()
	defer cancel()
	assert.NotNil(t, ctx)
	assert.NotNil(t, cancel)
	assert.NotNil(t, channel)
	cancelCtx := JobCancelContext(ctx)
	assert.NotNil(t, cancelCtx)
	assert.NoError(t, ctx.Err())
	assert.NoError(t, cancelCtx.Err())
	channel <- syscall.SIGTERM
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("context not canceled")
	case <-cancelCtx.Done():
	}
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("context not canceled")
	case <-ctx.Done():
	}
	if assert.Error(t, ctx.Err(), "context canceled") {
		assert.Equal(t, context.Canceled, ctx.Err())
	}
	if assert.Error(t, cancelCtx.Err(), "context canceled") {
		assert.Equal(t, context.Canceled, cancelCtx.Err())
	}
}

func TestCreateGracefulJobCancellationContext(t *testing.T) {
	ctx, cancel := CreateGracefulJobCancellationContext()
	defer cancel()
	assert.NotNil(t, ctx)
	assert.NotNil(t, cancel)
	cancelCtx := JobCancelContext(ctx)
	assert.NotNil(t, cancelCtx)
	assert.NoError(t, cancelCtx.Err())
}

func TestCreateGracefulJobCancellationContextCancelFunc(t *testing.T) {
	ctx, cancel := CreateGracefulJobCancellationContext()
	assert.NotNil(t, ctx)
	assert.NotNil(t, cancel)
	cancelCtx := JobCancelContext(ctx)
	assert.NotNil(t, cancelCtx)
	assert.NoError(t, cancelCtx.Err())
	cancel()
	if assert.Error(t, ctx.Err(), "context canceled") {
		assert.Equal(t, context.Canceled, ctx.Err())
	}
	if assert.Error(t, cancelCtx.Err(), "context canceled") {
		assert.Equal(t, context.Canceled, cancelCtx.Err())
	}
}
