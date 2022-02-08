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

type jobInfoMock struct {
	mock.Mock
}

func (jpm *jobInfoMock) matrix() map[string]interface{} {
	args := jpm.Called()
	return args.Get(0).(map[string]interface{})
}

func (jpm *jobInfoMock) steps() []*model.Step {
	args := jpm.Called()

	return args.Get(0).([]*model.Step)
}

func (jpm *jobInfoMock) startContainer() common.Executor {
	args := jpm.Called()

	return args.Get(0).(func(context.Context) error)
}

func (jpm *jobInfoMock) stopContainer() common.Executor {
	args := jpm.Called()

	return args.Get(0).(func(context.Context) error)
}

func (jpm *jobInfoMock) closeContainer() common.Executor {
	args := jpm.Called()

	return args.Get(0).(func(context.Context) error)
}

func (jpm *jobInfoMock) newStepExecutor(step *model.Step) common.Executor {
	args := jpm.Called(step)

	return args.Get(0).(func(context.Context) error)
}

func (jpm *jobInfoMock) interpolateOutputs() common.Executor {
	args := jpm.Called()

	return args.Get(0).(func(context.Context) error)
}

func (jpm *jobInfoMock) result(result string) {
	jpm.Called(result)
}

func TestNewJobExecutor(t *testing.T) {
	table := []struct {
		name     string
		steps    []*model.Step
		result   string
		hasError bool
	}{
		{
			"zeroSteps",
			[]*model.Step{},
			"success",
			false,
		},
		{
			"stepWithoutPrePost",
			[]*model.Step{{
				ID: "1",
			}},
			"success",
			false,
		},
		{
			"stepWithFailure",
			[]*model.Step{{
				ID: "1",
			}},
			"failure",
			true,
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctx := common.WithJobErrorContainer(context.Background())
			jpm := &jobInfoMock{}

			jpm.On("startContainer").Return(func(ctx context.Context) error {
				return nil
			})

			jpm.On("steps").Return(tt.steps)

			for _, stepMock := range tt.steps {
				jpm.On("newStepExecutor", stepMock).Return(func(ctx context.Context) error {
					if tt.hasError {
						return fmt.Errorf("error")
					}
					return nil
				})
			}

			jpm.On("interpolateOutputs").Return(func(ctx context.Context) error {
				return nil
			})

			jpm.On("matrix").Return(map[string]interface{}{})

			jpm.On("stopContainer").Return(func(ctx context.Context) error {
				return nil
			})

			jpm.On("result", tt.result)

			jpm.On("closeContainer").Return(func(ctx context.Context) error {
				return nil
			})

			executor := newJobExecutor(jpm)
			err := executor(ctx)
			assert.Nil(t, err)
		})
	}
}
