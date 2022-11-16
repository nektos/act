package runner

import (
	"context"
	"testing"

	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStepRun(t *testing.T) {
	cm := &containerMock{}
	fileEntry := &container.FileEntry{
		Name: "workflow/1.sh",
		Mode: 0755,
		Body: "\ncmd\n",
	}

	sr := &stepRun{
		RunContext: &RunContext{
			StepResults: map[string]*model.StepResult{},
			ExprEval:    &expressionEvaluator{},
			Config:      &Config{},
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
			ID:               "1",
			Run:              "cmd",
			WorkingDirectory: "workdir",
		},
	}

	cm.On("Copy", "/var/run/act", []*container.FileEntry{fileEntry}).Return(func(ctx context.Context) error {
		return nil
	})
	cm.On("Exec", []string{"bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "/var/run/act/workflow/1.sh"}, mock.AnythingOfType("map[string]string"), "", "workdir").Return(func(ctx context.Context) error {
		return nil
	})

	cm.On("UpdateFromImageEnv", mock.AnythingOfType("*map[string]string")).Return(func(ctx context.Context) error {
		return nil
	})

	cm.On("UpdateFromEnv", "/var/run/act/workflow/envs.txt", mock.AnythingOfType("*map[string]string")).Return(func(ctx context.Context) error {
		return nil
	})

	cm.On("UpdateFromPath", mock.AnythingOfType("*map[string]string")).Return(func(ctx context.Context) error {
		return nil
	})

	cm.On("Copy", "/var/run/act", mock.AnythingOfType("[]*container.FileEntry")).Return(func(ctx context.Context) error {
		return nil
	})

	cm.On("UpdateFromEnv", "/var/run/act/workflow/statecmd.txt", mock.AnythingOfType("*map[string]string")).Return(func(ctx context.Context) error {
		return nil
	})

	cm.On("UpdateFromEnv", "/var/run/act/workflow/outputcmd.txt", mock.AnythingOfType("*map[string]string")).Return(func(ctx context.Context) error {
		return nil
	})

	ctx := context.Background()

	err := sr.main()(ctx)
	assert.Nil(t, err)

	cm.AssertExpectations(t)
}

func TestStepRunPrePost(t *testing.T) {
	ctx := context.Background()
	sr := &stepRun{}

	err := sr.pre()(ctx)
	assert.Nil(t, err)

	err = sr.post()(ctx)
	assert.Nil(t, err)
}
