package runner

import (
	"context"
	"testing"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type stepActionLocalMocks struct {
	mock.Mock
}

func (salm *stepActionLocalMocks) runAction(step actionStep, actionDir string, remoteAction *remoteAction) common.Executor {
	args := salm.Called(step, actionDir, remoteAction)
	return args.Get(0).(func(context.Context) error)
}

func (salm *stepActionLocalMocks) readAction(step *model.Step, actionDir string, actionPath string, readFile actionYamlReader, writeFile fileWriter) (*model.Action, error) {
	args := salm.Called(step, actionDir, actionPath, readFile, writeFile)
	return args.Get(0).(*model.Action), args.Error(1)
}

func TestStepActionLocalTest(t *testing.T) {
	ctx := context.Background()

	cm := &containerMock{}
	salm := &stepActionLocalMocks{}

	sal := &stepActionLocal{
		readAction: salm.readAction,
		runAction:  salm.runAction,
		RunContext: &RunContext{
			StepResults: map[string]*model.StepResult{},
			ExprEval:    &expressionEvaluator{},
			Config: &Config{
				Workdir: "/tmp",
			},
			Run: &model.Run{
				JobID: "1",
				Workflow: &model.Workflow{
					Jobs: map[string]*model.Job{
						"1": {
							Defaults: model.Defaults{
								Run: model.RunDefaults{
									Shell: "bash",
								},
							},
						},
					},
				},
			},
			JobContainer: cm,
		},
		Step: &model.Step{
			ID:   "1",
			Uses: "./path/to/action",
		},
	}

	salm.On("readAction", sal.Step, "/tmp/path/to/action", "", mock.Anything, mock.Anything).
		Return(&model.Action{}, nil)

	cm.On("UpdateFromImageEnv", mock.AnythingOfType("*map[string]string")).Return(func(ctx context.Context) error {
		return nil
	})

	cm.On("UpdateFromEnv", "/var/run/act/workflow/envs.txt", mock.AnythingOfType("*map[string]string")).Return(func(ctx context.Context) error {
		return nil
	})

	cm.On("UpdateFromPath", mock.AnythingOfType("*map[string]string")).Return(func(ctx context.Context) error {
		return nil
	})

	salm.On("runAction", sal, "/tmp/path/to/action", (*remoteAction)(nil)).Return(func(ctx context.Context) error {
		return nil
	})

	err := sal.main()(ctx)

	assert.Nil(t, err)

	cm.AssertExpectations(t)
	salm.AssertExpectations(t)
}

func TestStepActionLocalPrePost(t *testing.T) {
	ctx := context.Background()

	sal := &stepActionLocal{}

	err := sal.pre()(ctx)
	assert.Nil(t, err)

	err = sal.post()(ctx)
	assert.Nil(t, err)
}
