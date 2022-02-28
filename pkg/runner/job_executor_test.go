package runner

import (
	"context"
	"fmt"
	"testing"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJobExecutor(t *testing.T) {
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

type jobInfoMock struct {
	mock.Mock
}

func (jim *jobInfoMock) matrix() map[string]interface{} {
	args := jim.Called()
	return args.Get(0).(map[string]interface{})
}

func (jim *jobInfoMock) steps() []*model.Step {
	args := jim.Called()

	return args.Get(0).([]*model.Step)
}

func (jim *jobInfoMock) startContainer() common.Executor {
	args := jim.Called()

	return args.Get(0).(func(context.Context) error)
}

func (jim *jobInfoMock) stopContainer() common.Executor {
	args := jim.Called()

	return args.Get(0).(func(context.Context) error)
}

func (jim *jobInfoMock) closeContainer() common.Executor {
	args := jim.Called()

	return args.Get(0).(func(context.Context) error)
}

func (jim *jobInfoMock) interpolateOutputs() common.Executor {
	args := jim.Called()

	return args.Get(0).(func(context.Context) error)
}

func (jim *jobInfoMock) result(result string) {
	jim.Called(result)
}

type stepFactoryMock struct {
	mock.Mock
}

func (sfm *stepFactoryMock) newStep(model *model.Step, rc *RunContext) (step, error) {
	args := sfm.Called(model, rc)
	return args.Get(0).(step), args.Error(1)
}

func TestNewJobExecutor(t *testing.T) {
	table := []struct {
		name          string
		steps         []*model.Step
		preSteps      []bool
		postSteps     []bool
		executedSteps []string
		result        string
		hasError      bool
	}{
		{
			name:          "zeroSteps",
			steps:         []*model.Step{},
			preSteps:      []bool{},
			postSteps:     []bool{},
			executedSteps: []string{},
			result:        "success",
			hasError:      false,
		},
		{
			name: "stepWithoutPrePost",
			steps: []*model.Step{{
				ID: "1",
			}},
			preSteps:  []bool{false},
			postSteps: []bool{false},
			executedSteps: []string{
				"startContainer",
				"step1",
				"stopContainer",
				"interpolateOutputs",
				"closeContainer",
			},
			result:   "success",
			hasError: false,
		},
		{
			name: "stepWithFailure",
			steps: []*model.Step{{
				ID: "1",
			}},
			preSteps:  []bool{false},
			postSteps: []bool{false},
			executedSteps: []string{
				"startContainer",
				"step1",
				"stopContainer",
				"interpolateOutputs",
				"closeContainer",
			},
			result:   "failure",
			hasError: true,
		},
		{
			name: "stepWithPre",
			steps: []*model.Step{{
				ID: "1",
			}},
			preSteps:  []bool{true},
			postSteps: []bool{false},
			executedSteps: []string{
				"startContainer",
				"pre1",
				"step1",
				"stopContainer",
				"interpolateOutputs",
				"closeContainer",
			},
			result:   "success",
			hasError: false,
		},
		{
			name: "stepWithPost",
			steps: []*model.Step{{
				ID: "1",
			}},
			preSteps:  []bool{false},
			postSteps: []bool{true},
			executedSteps: []string{
				"startContainer",
				"step1",
				"post1",
				"stopContainer",
				"interpolateOutputs",
				"closeContainer",
			},
			result:   "success",
			hasError: false,
		},
		{
			name: "stepWithPreAndPost",
			steps: []*model.Step{{
				ID: "1",
			}},
			preSteps:  []bool{true},
			postSteps: []bool{true},
			executedSteps: []string{
				"startContainer",
				"pre1",
				"step1",
				"post1",
				"stopContainer",
				"interpolateOutputs",
				"closeContainer",
			},
			result:   "success",
			hasError: false,
		},
		{
			name: "stepsWithPreAndPost",
			steps: []*model.Step{{
				ID: "1",
			}, {
				ID: "2",
			}, {
				ID: "3",
			}},
			preSteps:  []bool{true, false, true},
			postSteps: []bool{false, true, true},
			executedSteps: []string{
				"startContainer",
				"pre1",
				"pre3",
				"step1",
				"step2",
				"step3",
				"post3",
				"post2",
				"stopContainer",
				"interpolateOutputs",
				"closeContainer",
			},
			result:   "success",
			hasError: false,
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctx := common.WithJobErrorContainer(context.Background())
			jim := &jobInfoMock{}
			sfm := &stepFactoryMock{}
			rc := &RunContext{}
			executorOrder := make([]string, 0)

			jim.On("startContainer").Return(func(ctx context.Context) error {
				executorOrder = append(executorOrder, "startContainer")
				return nil
			})

			jim.On("steps").Return(tt.steps)

			for i, stepModel := range tt.steps {
				func(i int, stepModel *model.Step) {
					sm := &stepMock{}

					sfm.On("newStep", stepModel, rc).Return(sm, nil)

					sm.On("pre").Return(func(ctx context.Context) error {
						if tt.preSteps[i] {
							executorOrder = append(executorOrder, "pre"+stepModel.ID)
						}
						return nil
					})

					sm.On("main").Return(func(ctx context.Context) error {
						executorOrder = append(executorOrder, "step"+stepModel.ID)
						if tt.hasError {
							return fmt.Errorf("error")
						}
						return nil
					})

					sm.On("post").Return(func(ctx context.Context) error {
						if tt.postSteps[i] {
							executorOrder = append(executorOrder, "post"+stepModel.ID)
						}
						return nil
					})

					sm.AssertExpectations(t)
				}(i, stepModel)
			}

			jim.On("interpolateOutputs").Return(func(ctx context.Context) error {
				executorOrder = append(executorOrder, "interpolateOutputs")
				return nil
			})

			jim.On("matrix").Return(map[string]interface{}{})

			jim.On("stopContainer").Return(func(ctx context.Context) error {
				executorOrder = append(executorOrder, "stopContainer")
				return nil
			})

			jim.On("result", tt.result)

			jim.On("closeContainer").Return(func(ctx context.Context) error {
				executorOrder = append(executorOrder, "closeContainer")
				return nil
			})

			executor := newJobExecutor(jim, sfm, rc)
			err := executor(ctx)
			assert.Nil(t, err)
			assert.Equal(t, tt.executedSteps, executorOrder)

			jim.AssertExpectations(t)
			sfm.AssertExpectations(t)
		})
	}
}
