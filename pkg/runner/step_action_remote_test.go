package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/common/git"
	"github.com/nektos/act/pkg/model"
)

type stepActionRemoteMocks struct {
	mock.Mock
}

func (sarm *stepActionRemoteMocks) readAction(step *model.Step, actionDir string, actionPath string, readFile actionYamlReader, writeFile fileWriter) (*model.Action, error) {
	args := sarm.Called(step, actionDir, actionPath, readFile, writeFile)
	return args.Get(0).(*model.Action), args.Error(1)
}

func (sarm *stepActionRemoteMocks) runAction(step actionStep, actionDir string, remoteAction *remoteAction) common.Executor {
	args := sarm.Called(step, actionDir, remoteAction)
	return args.Get(0).(func(context.Context) error)
}

func TestStepActionRemoteTest(t *testing.T) {
	ctx := context.Background()

	cm := &containerMock{}

	sarm := &stepActionRemoteMocks{}

	clonedAction := false

	origStepAtionRemoteNewCloneExecutor := stepActionRemoteNewCloneExecutor
	stepActionRemoteNewCloneExecutor = func(input git.NewGitCloneExecutorInput) common.Executor {
		return func(ctx context.Context) error {
			clonedAction = true
			return nil
		}
	}
	defer (func() {
		stepActionRemoteNewCloneExecutor = origStepAtionRemoteNewCloneExecutor
	})()

	sar := &stepActionRemote{
		RunContext: &RunContext{
			Config: &Config{
				GitHubInstance: "github.com",
			},
			Run: &model.Run{
				JobID: "1",
				Workflow: &model.Workflow{
					Jobs: map[string]*model.Job{
						"1": {},
					},
				},
			},
			StepResults:  map[string]*model.StepResult{},
			JobContainer: cm,
		},
		Step: &model.Step{
			Uses: "remote/action@v1",
		},
		readAction: sarm.readAction,
		runAction:  sarm.runAction,
	}

	suffixMatcher := func(suffix string) interface{} {
		return mock.MatchedBy(func(actionDir string) bool {
			return strings.HasSuffix(actionDir, suffix)
		})
	}

	cm.On("UpdateFromImageEnv", &sar.env).Return(func(ctx context.Context) error { return nil })
	cm.On("UpdateFromEnv", "/var/run/act/workflow/envs.txt", &sar.env).Return(func(ctx context.Context) error { return nil })
	cm.On("UpdateFromPath", &sar.env).Return(func(ctx context.Context) error { return nil })

	sarm.On("readAction", sar.Step, suffixMatcher("act/remote-action@v1"), "", mock.Anything, mock.Anything).Return(&model.Action{}, nil)
	sarm.On("runAction", sar, suffixMatcher("act/remote-action@v1"), newRemoteAction(sar.Step.Uses)).Return(func(ctx context.Context) error { return nil })

	err := sar.main()(ctx)

	assert.Nil(t, err)
	assert.True(t, clonedAction)
	sarm.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStepActionRemotePrePost(t *testing.T) {
	ctx := context.Background()

	sar := &stepActionRemote{}

	err := sar.pre()(ctx)
	assert.Nil(t, err)

	err = sar.post()(ctx)
	assert.Nil(t, err)
}
