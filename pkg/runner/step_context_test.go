package runner

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

func TestStepContextExecutor(t *testing.T) {
	platforms := map[string]string{
		"ubuntu-latest": baseImage,
	}
	tables := []TestJobFileInfo{
		{"testdata", "uses-and-run-in-one-step", "push", "Invalid run/uses syntax for job:test step:Test", platforms, ""},
		{"testdata", "uses-github-empty", "push", "Expected format {org}/{repo}[/path]@ref", platforms, ""},
		{"testdata", "uses-github-noref", "push", "Expected format {org}/{repo}[/path]@ref", platforms, ""},
		{"testdata", "uses-github-root", "push", "", platforms, ""},
		{"testdata", "uses-github-path", "push", "", platforms, ""},
		{"testdata", "uses-docker-url", "push", "", platforms, ""},
		{"testdata", "uses-github-full-sha", "push", "", platforms, ""},
		{"testdata", "uses-github-short-sha", "push", "Unable to resolve action `actions/hello-world-docker-action@b136eb8`, the provided ref `b136eb8` is the shortened version of a commit SHA, which is not supported. Please use the full commit SHA `b136eb8894c5cb1dd5807da824be97ccdf9b5423` instead", platforms, ""},
	}
	// These tests are sufficient to only check syntax.
	ctx := common.WithDryrun(context.Background(), true)
	for _, table := range tables {
		runTestJobFile(ctx, t, table)
	}
}

func createIfTestStepContext(t *testing.T, input string) *StepContext {
	var step *model.Step
	err := yaml.Unmarshal([]byte(input), &step)
	assert.NoError(t, err)

	return &StepContext{
		RunContext: &RunContext{
			Config: &Config{
				Workdir: ".",
				Platforms: map[string]string{
					"ubuntu-latest": "ubuntu-latest",
				},
			},
			StepResults: map[string]*model.StepResult{},
			Env:         map[string]string{},
			Run: &model.Run{
				JobID: "job1",
				Workflow: &model.Workflow{
					Name: "workflow1",
					Jobs: map[string]*model.Job{
						"job1": createJob(t, `runs-on: ubuntu-latest`, ""),
					},
				},
			},
		},
		Step: step,
	}
}

func TestStepContextIsEnabled(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	assertObject := assert.New(t)

	// success()
	sc := createIfTestStepContext(t, "if: success()")
	assertObject.True(sc.isEnabled(context.Background()))

	sc = createIfTestStepContext(t, "if: success()")
	sc.RunContext.StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusSuccess,
	}
	assertObject.True(sc.isEnabled(context.Background()))

	sc = createIfTestStepContext(t, "if: success()")
	sc.RunContext.StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusFailure,
	}
	assertObject.False(sc.isEnabled(context.Background()))

	// failure()
	sc = createIfTestStepContext(t, "if: failure()")
	assertObject.False(sc.isEnabled(context.Background()))

	sc = createIfTestStepContext(t, "if: failure()")
	sc.RunContext.StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusSuccess,
	}
	assertObject.False(sc.isEnabled(context.Background()))

	sc = createIfTestStepContext(t, "if: failure()")
	sc.RunContext.StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusFailure,
	}
	assertObject.True(sc.isEnabled(context.Background()))

	// always()
	sc = createIfTestStepContext(t, "if: always()")
	assertObject.True(sc.isEnabled(context.Background()))

	sc = createIfTestStepContext(t, "if: always()")
	sc.RunContext.StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusSuccess,
	}
	assertObject.True(sc.isEnabled(context.Background()))

	sc = createIfTestStepContext(t, "if: always()")
	sc.RunContext.StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusFailure,
	}
	assertObject.True(sc.isEnabled(context.Background()))
}
