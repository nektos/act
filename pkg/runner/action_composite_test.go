package runner

import (
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestInvalidCompositeStepRequiresNoCleanup(t *testing.T) {
	run := &model.Run{
		JobID:    "test",
		Workflow: &model.Workflow{Jobs: map[string]*model.Job{"test": {}}},
	}
	rc := &RunContext{Run: run}
	action := &model.Action{Runs: model.ActionRuns{
		Steps: []model.Step{{Name: "something"}},
	}}

	steps := rc.compositeExecutor(action)
	assert.ErrorContains(t, steps.main(t.Context()), "Invalid run/uses syntax for job:test step:something")
	if assert.NotNil(t, steps.pre, "an invalid composite should have no pre-step error") {
		assert.NoError(t, steps.pre(t.Context()))
	}
	if assert.NotNil(t, steps.post, "an invalid composite should have no cleanup error") {
		assert.NoError(t, steps.post(t.Context()))
	}
}
