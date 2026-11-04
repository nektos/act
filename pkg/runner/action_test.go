package runner

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
					Using:  "node16",
					Main:   "main.js",
					PreIf:  "always()",
					PostIf: "always()",
				},
				ActionPath: "actionPath",
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
					Using:  "node16",
					Main:   "main.js",
					PreIf:  "always()",
					PostIf: "always()",
				},
				ActionPath: "actionPath",
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
				ActionPath: "actionPath",
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
				ActionPath: "actionPath",
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

			writeFile := func(filename string, _ []byte, perm fs.FileMode) error {
				assert.Equal(t, "actionDir/actionPath/trampoline.js", filename)
				assert.Equal(t, fs.FileMode(0400), perm)
				return nil
			}

			if tt.filename != "" {
				closerMock.On("Close")
			}

			action, err := readActionImpl(context.Background(), tt.step, "actionDir", "actionPath", readFile, writeFile)

			assert.Nil(t, err)
			assert.Equal(t, tt.expected, action)

			closerMock.AssertExpectations(t)
		})
	}
}

func TestActionReaderDiscoverSubdir(t *testing.T) {
	yaml := `
name: 'name'
runs:
  using: 'node16'
  main: 'main.js'
`

	baseDir := t.TempDir()
	subdir := filepath.Join(baseDir, "github-action")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "action.yml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write action.yml: %v", err)
	}

	readFile := func(filename string) (io.Reader, io.Closer, error) {
		return nil, nil, fs.ErrNotExist
	}

	writeFile := func(filename string, _ []byte, perm fs.FileMode) error {
		t.Fatalf("unexpected writeFile call: %s %v", filename, perm)
		return nil
	}

	action, err := readActionImpl(context.Background(), &model.Step{}, baseDir, "", readFile, writeFile)

	assert.Nil(t, err)
	assert.Equal(t, "github-action", action.ActionPath)
}

func TestActionRunner(t *testing.T) {
	table := []struct {
		name        string
		step        actionStep
		expectedEnv map[string]string
	}{
		{
			name: "with-input",
			step: &stepActionRemote{
				Step: &model.Step{
					Uses: "org/repo/path@ref",
				},
				RunContext: &RunContext{
					Config: &Config{},
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
					nodeToolFullPath: "node",
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
			expectedEnv: map[string]string{"INPUT_KEY": "default value"},
		},
		{
			name: "restore-saved-state",
			step: &stepActionRemote{
				Step: &model.Step{
					ID:   "step",
					Uses: "org/repo/path@ref",
				},
				RunContext: &RunContext{
					ActionPath: "path",
					Config:     &Config{},
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
					CurrentStep: "post-step",
					StepResults: map[string]*model.StepResult{
						"step": {},
					},
					IntraActionState: map[string]map[string]string{
						"step": {
							"name": "state value",
						},
					},
					nodeToolFullPath: "node",
				},
				action: &model.Action{
					Runs: model.ActionRuns{
						Using: "node16",
					},
				},
				env: map[string]string{},
			},
			expectedEnv: map[string]string{"STATE_name": "state value"},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			actionDir := fmt.Sprintf("%s/dir", tt.step.getRunContext().ActionCacheDir())

			cm := &containerMock{}
			cm.On("CopyDir", "/var/run/act/actions/dir/", actionDir+"/", false).Return(func(_ context.Context) error { return nil })

			envMatcher := mock.MatchedBy(func(env map[string]string) bool {
				for k, v := range tt.expectedEnv {
					if env[k] != v {
						return false
					}
				}
				return true
			})

			cm.On("Exec", []string{"node", "/var/run/act/actions/dir/path"}, envMatcher, "", "").Return(func(_ context.Context) error { return nil })

			tt.step.getRunContext().JobContainer = cm

			err := runActionImpl(tt.step, actionDir, newRemoteAction("org/repo/path@ref"))(ctx)

			assert.Nil(t, err)
			cm.AssertExpectations(t)
		})
	}
}

func TestActionReaderDiscoverSubdirMultipleMatches(t *testing.T) {
	yaml := `
name: 'name'
runs:
  using: 'node16'
  main: 'main.js'
`
	baseDir := t.TempDir()
	for _, dir := range []string{"github-action", "nested-action"} {
		subdir := filepath.Join(baseDir, dir)
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatalf("failed to create subdir %q: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(subdir, "action.yml"), []byte(yaml), 0o644); err != nil {
			t.Fatalf("failed to write action.yml in %q: %v", dir, err)
		}
	}

	readFile := func(filename string) (io.Reader, io.Closer, error) {
		return nil, nil, fs.ErrNotExist
	}

	writeFile := func(filename string, _ []byte, perm fs.FileMode) error {
		t.Fatalf("unexpected writeFile call: %s %v", filename, perm)
		return nil
	}

	action, err := readActionImpl(context.Background(), &model.Step{}, baseDir, "", readFile, writeFile)

	assert.Nil(t, action)
	if assert.Error(t, err) {
		assert.True(t,
			strings.Contains(strings.ToLower(err.Error()), "multiple") ||
				strings.Contains(strings.ToLower(err.Error()), "ambiguous"),
			"expected a clear error for multiple discovered action subdirectories, got: %v", err,
		)
	}
}

func TestActionReaderDiscoverSubdirActionCacheUnsupported(t *testing.T) {
	// When ActionCache is enabled, actionDir is a SHA hash (not a real path).
	// Discovery should fail with a clear, actionable error — not silently.
	nonExistentDir := filepath.Join(t.TempDir(), "sha-abc123def456")

	readFile := func(filename string) (io.Reader, io.Closer, error) {
		return nil, nil, fs.ErrNotExist
	}

	writeFile := func(filename string, _ []byte, perm fs.FileMode) error {
		t.Fatalf("unexpected writeFile call: %s %v", filename, perm)
		return nil
	}

	action, err := readActionImpl(context.Background(), &model.Step{}, nonExistentDir, "", readFile, writeFile)

	assert.Nil(t, action)
	if assert.Error(t, err) {
		assert.True(t,
			strings.Contains(strings.ToLower(err.Error()), "actioncache") ||
				strings.Contains(strings.ToLower(err.Error()), "tar") ||
				strings.Contains(strings.ToLower(err.Error()), "subdir") ||
				strings.Contains(strings.ToLower(err.Error()), "accessible"),
			"expected a clear error about ActionCache/discovery failure, got: %v", err,
		)
	}
}
