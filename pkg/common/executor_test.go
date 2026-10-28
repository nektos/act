package common

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

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
		func(_ context.Context) error {
			runcount++
			return nil
		},
		func(_ context.Context) error {
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

	err := NewConditionalExecutor(func(_ context.Context) bool {
		return false
	}, func(_ context.Context) error {
		trueCount++
		return nil
	}, func(_ context.Context) error {
		falseCount++
		return nil
	})(ctx)

	assert.Nil(err)
	assert.Equal(0, trueCount)
	assert.Equal(1, falseCount)

	err = NewConditionalExecutor(func(_ context.Context) bool {
		return true
	}, func(_ context.Context) error {
		trueCount++
		return nil
	}, func(_ context.Context) error {
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

	var count atomic.Int32
	var activeCount atomic.Int32
	var maxCount atomic.Int32
	emptyWorkflow := NewPipelineExecutor(func(_ context.Context) error {
		count.Add(1)

		current := activeCount.Add(1)
		for {
			seen := maxCount.Load()
			if current <= seen || maxCount.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(2 * time.Second)
		activeCount.Add(-1)

		return nil
	})

	err := NewParallelExecutor(2, emptyWorkflow, emptyWorkflow, emptyWorkflow)(ctx)

	assert.Equal(int32(3), count.Load(), "should run all 3 executors")
	assert.Equal(int32(2), maxCount.Load(), "should run at most 2 executors in parallel")
	assert.Nil(err)

	// Reset to test running the executor with 0 parallelism
	count.Store(0)
	activeCount.Store(0)
	maxCount.Store(0)

	errSingle := NewParallelExecutor(0, emptyWorkflow, emptyWorkflow, emptyWorkflow)(ctx)

	assert.Equal(int32(3), count.Load(), "should run all 3 executors")
	assert.Equal(int32(1), maxCount.Load(), "should run at most 1 executors in parallel")
	assert.Nil(errSingle)
}

func TestNewParallelExecutorFailed(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	count := 0
	errorWorkflow := NewPipelineExecutor(func(_ context.Context) error {
		count++
		return fmt.Errorf("fake error")
	})
	err := NewParallelExecutor(1, errorWorkflow)(ctx)
	assert.Equal(1, count)
	assert.ErrorIs(context.Canceled, err)
}

func TestNewParallelExecutorCanceled(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errExpected := fmt.Errorf("fake error")

	var count atomic.Int32
	successWorkflow := NewPipelineExecutor(func(_ context.Context) error {
		count.Add(1)
		return nil
	})
	errorWorkflow := NewPipelineExecutor(func(_ context.Context) error {
		count.Add(1)
		return errExpected
	})
	err := NewParallelExecutor(3, errorWorkflow, successWorkflow, successWorkflow)(ctx)
	assert.Equal(int32(3), count.Load())
	assert.Error(errExpected, err)
}
