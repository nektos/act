package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWorkflow(t *testing.T) {
	assert := assert.New(t)

	// empty
	emptyWorkflow := NewPipelineExecutor()
	assert.Nil(emptyWorkflow())

	// error case
	errorWorkflow := NewErrorExecutor(fmt.Errorf("test error"))
	assert.NotNil(errorWorkflow())

	// multiple success case
	runcount := 0
	successWorkflow := NewPipelineExecutor(
		func() error {
			runcount ++
			return nil
		},
		func() error {
			runcount ++
			return nil
		})
	assert.Nil(successWorkflow())
	assert.Equal(2, runcount)
}

func TestNewConditionalExecutor(t *testing.T) {
	assert := assert.New(t)

	trueCount := 0
	falseCount := 0

	err := NewConditionalExecutor(func() bool {
		return false
	}, func() error {
		trueCount++
		return nil
	}, func() error {
		falseCount++
		return nil
	})()

	assert.Nil(err)
	assert.Equal(0, trueCount)
	assert.Equal(1, falseCount)

	err = NewConditionalExecutor(func() bool {
		return true
	}, func() error {
		trueCount++
		return nil
	}, func() error {
		falseCount++
		return nil
	})()

	assert.Nil(err)
	assert.Equal(1, trueCount)
	assert.Equal(1, falseCount)
}

func TestNewParallelExecutor(t *testing.T) {
	assert := assert.New(t)

	count := 0
	emptyWorkflow := NewPipelineExecutor(func() error {
		count++
		return nil
	})

	err := NewParallelExecutor(emptyWorkflow, emptyWorkflow)()
	assert.Equal(2, count)

	assert.Nil(err)
}
