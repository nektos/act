package runner

import (
	"context"
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateCompositeInputsAndEnvAreIsolated(t *testing.T) {
	runContext := &RunContext{
		Config: &Config{
			Env: map[string]string{
				"GITHUB_REF":        "refs/heads/main",
				"GITHUB_REPOSITORY": "owner/repository",
				"SHA_REF":           "0123456789abcdef0123456789abcdef01234567",
			},
			GitHubInstance: "github.com",
			Workdir:        t.TempDir(),
		},
		Env: map[string]string{},
		Run: &model.Run{
			JobID: "test",
			Workflow: &model.Workflow{
				Name: "composite input context",
				Jobs: map[string]*model.Job{"test": {}},
			},
		},
		StepResults: map[string]*model.StepResult{},
	}
	step := &stepActionLocal{
		Step: &model.Step{
			Uses: "./composite",
			With: map[string]string{"explicit": "from-workflow"},
		},
		RunContext: runContext,
		action: &model.Action{
			Inputs: map[string]model.Input{
				"explicit": {Required: true},
				"fallback": {Default: "fallback-value"},
			},
		},
		env: map[string]string{
			"INPUT_EXPLICIT": "from-workflow",
			"JOB_ENV":        "preserved",
		},
	}

	ctx := context.Background()
	env := evaluateCompositeEnv(ctx, step)
	inputs := evaluateCompositeInputs(ctx, runContext, step)

	assert.Equal(t, "preserved", env["JOB_ENV"])
	assert.NotContains(t, env, "INPUT_EXPLICIT")
	assert.NotContains(t, env, "INPUT_FALLBACK")
	assert.Equal(t, map[string]interface{}{
		"explicit": "from-workflow",
		"fallback": "fallback-value",
	}, inputs)
}

func TestCompositeActionInputsStayInExpressionContext(t *testing.T) {
	runContext := &RunContext{
		ActionInputs: map[string]interface{}{
			"parent": "parent-value",
			"shared": "parent-shared",
		},
		Config: &Config{
			Env: map[string]string{
				"GITHUB_REF":        "refs/heads/main",
				"GITHUB_REPOSITORY": "owner/repository",
				"SHA_REF":           "0123456789abcdef0123456789abcdef01234567",
			},
			GitHubInstance: "github.com",
			Workdir:        t.TempDir(),
		},
		Env: map[string]string{},
		Run: &model.Run{
			JobID: "test",
			Workflow: &model.Workflow{
				Name: "composite input context",
				Jobs: map[string]*model.Job{"test": {}},
			},
		},
		StepResults: map[string]*model.StepResult{},
	}
	step := &stepRun{
		Step:       &model.Step{},
		RunContext: runContext,
		env:        map[string]string{},
	}

	evaluator := runContext.NewStepExpressionEvaluator(context.Background(), step)
	assert.Equal(t, "parent-value", evaluator.Interpolate(context.Background(), "${{ inputs.parent }}"))
	assert.NotContains(t, *step.getEnv(), "INPUT_PARENT")

	step.env["INPUT_SHARED"] = "child-shared"
	step.env["INPUT_CHILD"] = "child-value"
	inputs := getEvaluatorInputs(context.Background(), runContext, step, &model.GithubContext{})
	assert.Equal(t, map[string]interface{}{
		"child":  "child-value",
		"parent": "parent-value",
		"shared": "child-shared",
	}, inputs)
}
