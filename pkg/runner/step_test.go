package runner

import (
	"context"
	"testing"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	yaml "gopkg.in/yaml.v3"
)

func TestMergeIntoMap(t *testing.T) {
	table := []struct {
		name     string
		target   map[string]string
		maps     []map[string]string
		expected map[string]string
	}{
		{
			name:     "testEmptyMap",
			target:   map[string]string{},
			maps:     []map[string]string{},
			expected: map[string]string{},
		},
		{
			name:   "testMergeIntoEmptyMap",
			target: map[string]string{},
			maps: []map[string]string{
				{
					"key1": "value1",
					"key2": "value2",
				}, {
					"key2": "overridden",
					"key3": "value3",
				},
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "overridden",
				"key3": "value3",
			},
		},
		{
			name: "testMergeIntoExistingMap",
			target: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			maps: []map[string]string{
				{
					"key1": "overridden",
				},
			},
			expected: map[string]string{
				"key1": "overridden",
				"key2": "value2",
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			mergeIntoMapCaseSensitive(tt.target, tt.maps...)
			assert.Equal(t, tt.expected, tt.target)
			mergeIntoMapCaseInsensitive(tt.target, tt.maps...)
			assert.Equal(t, tt.expected, tt.target)
		})
	}
}

type stepMock struct {
	mock.Mock
	step
}

func (sm *stepMock) pre() common.Executor {
	args := sm.Called()
	return args.Get(0).(func(context.Context) error)
}

func (sm *stepMock) main() common.Executor {
	args := sm.Called()
	return args.Get(0).(func(context.Context) error)
}

func (sm *stepMock) post() common.Executor {
	args := sm.Called()
	return args.Get(0).(func(context.Context) error)
}

func (sm *stepMock) getRunContext() *RunContext {
	args := sm.Called()
	return args.Get(0).(*RunContext)
}

func (sm *stepMock) getGithubContext(ctx context.Context) *model.GithubContext {
	args := sm.Called()
	return args.Get(0).(*RunContext).getGithubContext(ctx)
}

func (sm *stepMock) getStepModel() *model.Step {
	args := sm.Called()
	return args.Get(0).(*model.Step)
}

func (sm *stepMock) getEnv() *map[string]string {
	args := sm.Called()
	return args.Get(0).(*map[string]string)
}

func TestSetupEnv(t *testing.T) {
	cm := &containerMock{}
	sm := &stepMock{}

	rc := &RunContext{
		Config: &Config{
			Env: map[string]string{
				"GITHUB_RUN_ID": "runId",
			},
		},
		Run: &model.Run{
			JobID: "1",
			Workflow: &model.Workflow{
				Jobs: map[string]*model.Job{
					"1": {
						Env: yaml.Node{
							Value: "JOB_KEY: jobvalue",
						},
					},
				},
			},
		},
		Env: map[string]string{
			"RC_KEY": "rcvalue",
		},
		JobContainer: cm,
	}
	step := &model.Step{
		With: map[string]string{
			"STEP_WITH": "with-value",
		},
	}
	env := map[string]string{}

	sm.On("getRunContext").Return(rc)
	sm.On("getGithubContext").Return(rc)
	sm.On("getStepModel").Return(step)
	sm.On("getEnv").Return(&env)

	err := setupEnv(context.Background(), sm)
	assert.Nil(t, err)

	// These are commit or system specific
	delete((env), "GITHUB_REF")
	delete((env), "GITHUB_REF_NAME")
	delete((env), "GITHUB_REF_TYPE")
	delete((env), "GITHUB_SHA")
	delete((env), "GITHUB_WORKSPACE")
	delete((env), "GITHUB_REPOSITORY")
	delete((env), "GITHUB_REPOSITORY_OWNER")
	delete((env), "GITHUB_ACTOR")

	assert.Equal(t, map[string]string{
		"ACT":                      "true",
		"CI":                       "true",
		"GITHUB_ACTION":            "",
		"GITHUB_ACTIONS":           "true",
		"GITHUB_ACTION_PATH":       "",
		"GITHUB_ACTION_REF":        "",
		"GITHUB_ACTION_REPOSITORY": "",
		"GITHUB_API_URL":           "https:///api/v3",
		"GITHUB_BASE_REF":          "",
		"GITHUB_EVENT_NAME":        "",
		"GITHUB_EVENT_PATH":        "/var/run/act/workflow/event.json",
		"GITHUB_GRAPHQL_URL":       "https:///api/graphql",
		"GITHUB_HEAD_REF":          "",
		"GITHUB_JOB":               "1",
		"GITHUB_RETENTION_DAYS":    "0",
		"GITHUB_RUN_ID":            "runId",
		"GITHUB_RUN_NUMBER":        "1",
		"GITHUB_SERVER_URL":        "https://",
		"GITHUB_WORKFLOW":          "",
		"INPUT_STEP_WITH":          "with-value",
		"RC_KEY":                   "rcvalue",
		"RUNNER_PERFLOG":           "/dev/null",
		"RUNNER_TRACKING_ID":       "",
	}, env)

	cm.AssertExpectations(t)
}

func TestIsStepEnabled(t *testing.T) {
	createTestStep := func(t *testing.T, input string) step {
		var step *model.Step
		err := yaml.Unmarshal([]byte(input), &step)
		assert.NoError(t, err)

		return &stepRun{
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

	log.SetLevel(log.DebugLevel)
	assertObject := assert.New(t)

	// success()
	step := createTestStep(t, "if: success()")
	assertObject.True(isStepEnabled(context.Background(), step.getIfExpression(context.Background(), stepStageMain), step, stepStageMain))

	step = createTestStep(t, "if: success()")
	step.getRunContext().StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusSuccess,
	}
	assertObject.True(isStepEnabled(context.Background(), step.getStepModel().If.Value, step, stepStageMain))

	step = createTestStep(t, "if: success()")
	step.getRunContext().StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusFailure,
	}
	assertObject.False(isStepEnabled(context.Background(), step.getStepModel().If.Value, step, stepStageMain))

	// failure()
	step = createTestStep(t, "if: failure()")
	assertObject.False(isStepEnabled(context.Background(), step.getStepModel().If.Value, step, stepStageMain))

	step = createTestStep(t, "if: failure()")
	step.getRunContext().StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusSuccess,
	}
	assertObject.False(isStepEnabled(context.Background(), step.getStepModel().If.Value, step, stepStageMain))

	step = createTestStep(t, "if: failure()")
	step.getRunContext().StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusFailure,
	}
	assertObject.True(isStepEnabled(context.Background(), step.getStepModel().If.Value, step, stepStageMain))

	// always()
	step = createTestStep(t, "if: always()")
	assertObject.True(isStepEnabled(context.Background(), step.getStepModel().If.Value, step, stepStageMain))

	step = createTestStep(t, "if: always()")
	step.getRunContext().StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusSuccess,
	}
	assertObject.True(isStepEnabled(context.Background(), step.getStepModel().If.Value, step, stepStageMain))

	step = createTestStep(t, "if: always()")
	step.getRunContext().StepResults["a"] = &model.StepResult{
		Conclusion: model.StepStatusFailure,
	}
	assertObject.True(isStepEnabled(context.Background(), step.getStepModel().If.Value, step, stepStageMain))
}

func TestIsContinueOnError(t *testing.T) {
	createTestStep := func(t *testing.T, input string) step {
		var step *model.Step
		err := yaml.Unmarshal([]byte(input), &step)
		assert.NoError(t, err)

		return &stepRun{
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

	log.SetLevel(log.DebugLevel)
	assertObject := assert.New(t)

	// absent
	step := createTestStep(t, "name: test")
	continueOnError, err := isContinueOnError(context.Background(), step.getStepModel().RawContinueOnError, step, stepStageMain)
	assertObject.False(continueOnError)
	assertObject.Nil(err)

	// explcit true
	step = createTestStep(t, "continue-on-error: true")
	continueOnError, err = isContinueOnError(context.Background(), step.getStepModel().RawContinueOnError, step, stepStageMain)
	assertObject.True(continueOnError)
	assertObject.Nil(err)

	// explicit false
	step = createTestStep(t, "continue-on-error: false")
	continueOnError, err = isContinueOnError(context.Background(), step.getStepModel().RawContinueOnError, step, stepStageMain)
	assertObject.False(continueOnError)
	assertObject.Nil(err)

	// expression true
	step = createTestStep(t, "continue-on-error: ${{ 'test' == 'test' }}")
	continueOnError, err = isContinueOnError(context.Background(), step.getStepModel().RawContinueOnError, step, stepStageMain)
	assertObject.True(continueOnError)
	assertObject.Nil(err)

	// expression false
	step = createTestStep(t, "continue-on-error: ${{ 'test' != 'test' }}")
	continueOnError, err = isContinueOnError(context.Background(), step.getStepModel().RawContinueOnError, step, stepStageMain)
	assertObject.False(continueOnError)
	assertObject.Nil(err)

	// expression parse error
	step = createTestStep(t, "continue-on-error: ${{ 'test' != test }}")
	continueOnError, err = isContinueOnError(context.Background(), step.getStepModel().RawContinueOnError, step, stepStageMain)
	assertObject.False(continueOnError)
	assertObject.NotNil(err)
}
