package runner

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
)

// TestMatrixJobOutputsMerge tests that outputs from parallel matrix jobs are properly merged
func TestMatrixJobOutputsMerge(t *testing.T) {
	// Create a job with outputs that will be set by different matrix runs
	job := &model.Job{
		Outputs: map[string]string{
			"v1var": "${{ steps.step1.outputs.v1 }}",
			"v2var": "${{ steps.step1.outputs.v2 }}",
		},
	}

	workflow := &model.Workflow{
		Jobs: map[string]*model.Job{
			"test": job,
		},
	}

	plan := &model.Plan{
		Stages: []*model.Stage{
			{
				Runs: []*model.Run{
					{
						Workflow: workflow,
						JobID:    "test",
					},
				},
			},
		},
	}

	// Create first run context (matrix.foo = v1)
	rc1 := &RunContext{
		Config:    &Config{},
		Run:       plan.Stages[0].Runs[0],
		EventJSON: "{}",
		StepResults: map[string]*model.StepResult{
			"step1": {
				Outputs: map[string]string{
					"v1": "v1", // Only v1 output is set
				},
			},
		},
	}
	rc1.ExprEval = rc1.NewExpressionEvaluator(context.Background())

	// Create second run context (matrix.foo = v2)
	rc2 := &RunContext{
		Config:    &Config{},
		Run:       plan.Stages[0].Runs[0],
		EventJSON: "{}",
		StepResults: map[string]*model.StepResult{
			"step1": {
				Outputs: map[string]string{
					"v2": "v2", // Only v2 output is set
				},
			},
		},
	}
	rc2.ExprEval = rc2.NewExpressionEvaluator(context.Background())

	// Simulate parallel execution by calling interpolateOutputs from both contexts
	ctx := context.Background()

	// First matrix run sets v1var
	err := rc1.interpolateOutputs()(ctx)
	assert.NoError(t, err)

	// Second matrix run sets v2var
	err = rc2.interpolateOutputs()(ctx)
	assert.NoError(t, err)

	// Verify that both outputs are set (merged from both matrix runs)
	assert.Equal(t, "v1", job.Outputs["v1var"], "v1var should be set from first matrix run")
	assert.Equal(t, "v2", job.Outputs["v2var"], "v2var should be set from second matrix run")
}

// TestMatrixJobOutputsParallelWithDelay tests that outputs from parallel matrix jobs
// with random delays are properly merged without race conditions
func TestMatrixJobOutputsParallelWithDelay(t *testing.T) {
	// Create a job with outputs that will be set by different matrix runs
	job := &model.Job{
		Outputs: map[string]string{
			"v1var": "${{ steps.step1.outputs.v1 }}",
			"v2var": "${{ steps.step1.outputs.v2 }}",
		},
	}

	workflow := &model.Workflow{
		Jobs: map[string]*model.Job{
			"test": job,
		},
	}

	plan := &model.Plan{
		Stages: []*model.Stage{
			{
				Runs: []*model.Run{
					{
						Workflow: workflow,
						JobID:    "test",
					},
				},
			},
		},
	}

	// Create contexts for two parallel matrix runs
	contexts := []*RunContext{
		// Matrix run 1: sets v1 output
		{
			Config:    &Config{},
			Run:       plan.Stages[0].Runs[0],
			EventJSON: "{}",
			StepResults: map[string]*model.StepResult{
				"step1": {
					Outputs: map[string]string{
						"v1": "v1",
					},
				},
			},
		},
		// Matrix run 2: sets v2 output
		{
			Config:    &Config{},
			Run:       plan.Stages[0].Runs[0],
			EventJSON: "{}",
			StepResults: map[string]*model.StepResult{
				"step1": {
					Outputs: map[string]string{
						"v2": "v2",
					},
				},
			},
		},
	}

	// Initialize expression evaluators
	for _, rc := range contexts {
		rc.ExprEval = rc.NewExpressionEvaluator(context.Background())
	}

	// Use WaitGroup to ensure both goroutines complete
	var wg sync.WaitGroup
	var errors []error
	var errorsMu sync.Mutex

	ctx := context.Background()

	// Launch parallel matrix runs with random delays
	for i, rc := range contexts {
		wg.Add(1)
		go func(index int, runContext *RunContext) {
			defer wg.Done()

			// Random delay between 5 and 10 seconds
			delaySeconds := 5 + rand.Intn(6)
			t.Logf("Matrix run %d: sleeping for %d seconds", index+1, delaySeconds)
			time.Sleep(time.Duration(delaySeconds) * time.Second)

			// Interpolate outputs
			t.Logf("Matrix run %d: interpolating outputs", index+1)
			if err := runContext.interpolateOutputs()(ctx); err != nil {
				errorsMu.Lock()
				errors = append(errors, err)
				errorsMu.Unlock()
			}
			t.Logf("Matrix run %d: completed", index+1)
		}(i, rc)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check for errors
	assert.Empty(t, errors, "No errors should occur during parallel execution")

	// Verify that both outputs are set (merged from both matrix runs)
	assert.Equal(t, "v1", job.Outputs["v1var"], "v1var should be set from first matrix run")
	assert.Equal(t, "v2", job.Outputs["v2var"], "v2var should be set from second matrix run")

	// Verify RawOutputs are preserved
	assert.NotNil(t, job.RawOutputs, "RawOutputs should be initialized")
	assert.Equal(t, "${{ steps.step1.outputs.v1 }}", job.RawOutputs["v1var"], "RawOutputs should preserve original templates")
	assert.Equal(t, "${{ steps.step1.outputs.v2 }}", job.RawOutputs["v2var"], "RawOutputs should preserve original templates")
}
