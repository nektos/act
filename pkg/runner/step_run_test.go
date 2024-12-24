package runner

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
)

func TestStepRun(t *testing.T) {
	cm := &containerMock{}
	fileEntry := &container.FileEntry{
		Name: "workflow/1.sh",
		Mode: 0o755,
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

	cm.On("Copy", "/var/run/act", []*container.FileEntry{fileEntry}).Return(func(_ context.Context) error {
		return nil
	})
	cm.On("Exec", []string{"bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "/var/run/act/workflow/1.sh"}, mock.AnythingOfType("map[string]string"), "", "workdir").Return(func(_ context.Context) error {
		return nil
	})

	cm.On("Copy", "/var/run/act", mock.AnythingOfType("[]*container.FileEntry")).Return(func(_ context.Context) error {
		return nil
	})

	cm.On("UpdateFromEnv", "/var/run/act/workflow/envs.txt", mock.AnythingOfType("*map[string]string")).Return(func(_ context.Context) error {
		return nil
	})

	cm.On("UpdateFromEnv", "/var/run/act/workflow/statecmd.txt", mock.AnythingOfType("*map[string]string")).Return(func(_ context.Context) error {
		return nil
	})

	cm.On("UpdateFromEnv", "/var/run/act/workflow/outputcmd.txt", mock.AnythingOfType("*map[string]string")).Return(func(_ context.Context) error {
		return nil
	})

	ctx := context.Background()

	cm.On("GetContainerArchive", ctx, "/var/run/act/workflow/pathcmd.txt").Return(io.NopCloser(&bytes.Buffer{}), nil)

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
