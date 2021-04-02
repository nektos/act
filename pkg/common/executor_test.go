package common

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWorkflow(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()

	// empty
	emptyWorkflow := NewPipelineExecutor()
	assert.Nil(emptyWorkflow(ctx))

	// error case
	errorWorkflow := NewErrorExecutor(fmt.Errorf("test error"))
	assert.NotNil(errorWorkflow(ctx))

	// multiple success case
	runcount := 0
	successWorkflow := NewPipelineExecutor(
		func(ctx context.Context) error {
			runcount++
			return nil
		},
		func(ctx context.Context) error {
			runcount++
			return nil
		})
	assert.Nil(successWorkflow(ctx))
	assert.Equal(2, runcount)
}

func TestNewConditionalExecutor(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()

	trueCount := 0
	falseCount := 0

	err := NewConditionalExecutor(func(ctx context.Context) bool {
		return false
	}, func(ctx context.Context) error {
		trueCount++
		return nil
	}, func(ctx context.Context) error {
		falseCount++
		return nil
	})(ctx)

	assert.Nil(err)
	assert.Equal(0, trueCount)
	assert.Equal(1, falseCount)

	err = NewConditionalExecutor(func(ctx context.Context) bool {
		return true
	}, func(ctx context.Context) error {
		trueCount++
		return nil
	}, func(ctx context.Context) error {
		falseCount++
		return nil
	})(ctx)

	assert.Nil(err)
	assert.Equal(1, trueCount)
	assert.Equal(1, falseCount)
}

func TestNewParallelExecutor(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()

	count := 0
	emptyWorkflow := NewPipelineExecutor(func(ctx context.Context) error {
		count++
		return nil
	})

	err := NewParallelExecutor(emptyWorkflow, emptyWorkflow)(ctx)
	assert.Equal(2, count)

	assert.Nil(err)
}

func TestNewParallelExecutorFailed(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	count := 0
	errorWorkflow := NewPipelineExecutor(func(ctx context.Context) error {
		count++
		return fmt.Errorf("fake error")
	})
	err := NewParallelExecutor(errorWorkflow)(ctx)
	assert.Equal(1, count)
	assert.ErrorIs(context.Canceled, err)
}

func TestNewParallelExecutorCanceled(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errExpected := fmt.Errorf("fake error")

	count := 0
	successWorkflow := NewPipelineExecutor(func(ctx context.Context) error {
		count++
		return nil
	})
	errorWorkflow := NewPipelineExecutor(func(ctx context.Context) error {
		count++
		return errExpected
	})
	err := NewParallelExecutor(errorWorkflow, successWorkflow, successWorkflow)(ctx)
	assert.Equal(3, count)
	assert.Error(errExpected, err)
}
