package runner

import (
	"context"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type closerMock struct {
	mock.Mock
}

func (m *closerMock) Close() error {
	m.Called()
	return nil
}

func TestActionReader(t *testing.T) {
	yaml := strings.ReplaceAll(`
name: 'name'
runs:
  using: 'node16'
  main: 'main.js'
`, "\t", "  ")

	table := []struct {
		name        string
		step        *model.Step
		filename    string
		fileContent string
		expected    *model.Action
	}{
		{
			name:        "readActionYml",
			step:        &model.Step{},
			filename:    "action.yml",
			fileContent: yaml,
			expected: &model.Action{
				Name: "name",
				Runs: model.ActionRuns{
					Using: "node16",
					Main:  "main.js",
				},
			},
		},
		{
			name:        "readActionYaml",
			step:        &model.Step{},
			filename:    "action.yaml",
			fileContent: yaml,
			expected: &model.Action{
				Name: "name",
				Runs: model.ActionRuns{
					Using: "node16",
					Main:  "main.js",
				},
			},
		},
		{
			name:        "readDockerfile",
			step:        &model.Step{},
			filename:    "Dockerfile",
			fileContent: "FROM ubuntu:20.04",
			expected: &model.Action{
				Name: "(Synthetic)",
				Runs: model.ActionRuns{
					Using: "docker",
					Image: "Dockerfile",
				},
			},
		},
		{
			name: "readWithArgs",
			step: &model.Step{
				With: map[string]string{
					"args": "cmd",
				},
			},
			expected: &model.Action{
				Name: "(Synthetic)",
				Inputs: map[string]model.Input{
					"cwd": {
						Description: "(Actual working directory)",
						Required:    false,
						Default:     "actionDir/actionPath",
					},
					"command": {
						Description: "(Actual program)",
						Required:    false,
						Default:     "cmd",
					},
				},
				Runs: model.ActionRuns{
					Using: "node12",
					Main:  "trampoline.js",
				},
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			closerMock := &closerMock{}

			readFile := func(filename string) (io.Reader, io.Closer, error) {
				if tt.filename != filename {
					return nil, nil, fs.ErrNotExist
				}

				return strings.NewReader(tt.fileContent), closerMock, nil
			}

			writeFile := func(filename string, data []byte, perm fs.FileMode) error {
				assert.Equal(t, "actionDir/actionPath/trampoline.js", filename)
				assert.Equal(t, fs.FileMode(0400), perm)
				return nil
			}

			closerMock.On("Close")

			action, err := readActionImpl(tt.step, "actionDir", "actionPath", readFile, writeFile)

			assert.Nil(t, err)
			assert.Equal(t, tt.expected, action)
		})
	}
}

type exprEvalMock struct {
	ExpressionEvaluator
	mock.Mock
}

func (e *exprEvalMock) Interpolate(expr string) string {
	args := e.Called(expr)
	return args.String(0)
}

func TestActionRunner(t *testing.T) {
	table := []struct {
		name string
		step actionStep
	}{
		{
			name: "Test",
			step: &stepActionRemote{
				Step: &model.Step{
					Uses: "repo@ref",
				},
				RunContext: &RunContext{
					ActionRepository: "repo",
					ActionPath:       "path",
					ActionRef:        "ref",
					Config:           &Config{},
					Run: &model.Run{
						JobID: "job",
						Workflow: &model.Workflow{
							Jobs: map[string]*model.Job{
								"job": {
									Name: "job",
								},
							},
						},
					},
				},
				action: &model.Action{
					Inputs: map[string]model.Input{
						"key": {
							Default: "default value",
						},
					},
					Runs: model.ActionRuns{
						Using: "node16",
					},
				},
				env: map[string]string{},
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			cm := &containerMock{}
			cm.On("CopyDir", "/var/run/act/actions/dir/", "dir/", false).Return(func(ctx context.Context) error { return nil })
			cm.On("Exec", []string{"node", "/var/run/act/actions/dir/path"}, map[string]string{"INPUT_KEY": "default value"}, "", "").Return(func(ctx context.Context) error { return nil })
			tt.step.getRunContext().JobContainer = cm

			ee := &exprEvalMock{}
			ee.On("Interpolate", "default value").Return("default value")
			tt.step.getRunContext().ExprEval = ee

			_, localAction := tt.step.(*stepActionRemote)

			err := runActionImpl(tt.step, "dir", "path", "repo", "ref", localAction)(ctx)

			assert.Nil(t, err)
			ee.AssertExpectations(t)
			cm.AssertExpectations(t)
		})
	}
}
