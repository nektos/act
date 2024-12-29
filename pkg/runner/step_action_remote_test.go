package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/common/git"
	"github.com/nektos/act/pkg/model"
)

type stepActionRemoteMocks struct {
	mock.Mock
}

func (sarm *stepActionRemoteMocks) readAction(_ context.Context, step *model.Step, actionDir string, actionPath string, readFile actionYamlReader, writeFile fileWriter) (*model.Action, error) {
	args := sarm.Called(step, actionDir, actionPath, readFile, writeFile)
	return args.Get(0).(*model.Action), args.Error(1)
}

func (sarm *stepActionRemoteMocks) runAction(step actionStep, actionDir string, remoteAction *remoteAction) common.Executor {
	args := sarm.Called(step, actionDir, remoteAction)
	return args.Get(0).(func(context.Context) error)
}

func TestStepActionRemote(t *testing.T) {
	table := []struct {
		name      string
		stepModel *model.Step
		result    *model.StepResult
		mocks     struct {
			env    bool
			cloned bool
			read   bool
			run    bool
		}
		runError error
	}{
		{
			name: "run-successful",
			stepModel: &model.Step{
				ID:   "step",
				Uses: "remote/action@v1",
			},
			result: &model.StepResult{
				Conclusion: model.StepStatusSuccess,
				Outcome:    model.StepStatusSuccess,
				Outputs:    map[string]string{},
			},
			mocks: struct {
				env    bool
				cloned bool
				read   bool
				run    bool
			}{
				env:    true,
				cloned: true,
				read:   true,
				run:    true,
			},
		},
		{
			name: "run-skipped",
			stepModel: &model.Step{
				ID:   "step",
				Uses: "remote/action@v1",
				If:   yaml.Node{Value: "false"},
			},
			result: &model.StepResult{
				Conclusion: model.StepStatusSkipped,
				Outcome:    model.StepStatusSkipped,
				Outputs:    map[string]string{},
			},
			mocks: struct {
				env    bool
				cloned bool
				read   bool
				run    bool
			}{
				env:    true,
				cloned: true,
				read:   true,
				run:    false,
			},
		},
		{
			name: "run-error",
			stepModel: &model.Step{
				ID:   "step",
				Uses: "remote/action@v1",
			},
			result: &model.StepResult{
				Conclusion: model.StepStatusFailure,
				Outcome:    model.StepStatusFailure,
				Outputs:    map[string]string{},
			},
			mocks: struct {
				env    bool
				cloned bool
				read   bool
				run    bool
			}{
				env:    true,
				cloned: true,
				read:   true,
				run:    true,
			},
			runError: errors.New("error"),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			cm := &containerMock{}
			sarm := &stepActionRemoteMocks{}

			clonedAction := false

			origStepAtionRemoteNewCloneExecutor := stepActionRemoteNewCloneExecutor
			stepActionRemoteNewCloneExecutor = func(_ git.NewGitCloneExecutorInput) common.Executor {
				return func(_ context.Context) error {
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
				Step:       tt.stepModel,
				readAction: sarm.readAction,
				runAction:  sarm.runAction,
			}
			sar.RunContext.ExprEval = sar.RunContext.NewExpressionEvaluator(ctx)

			suffixMatcher := func(suffix string) interface{} {
				return mock.MatchedBy(func(actionDir string) bool {
					return strings.HasSuffix(actionDir, suffix)
				})
			}

			if tt.mocks.read {
				sarm.On("readAction", sar.Step, suffixMatcher("act/remote-action@v1"), "", mock.Anything, mock.Anything).Return(&model.Action{}, nil)
			}
			if tt.mocks.run {
				sarm.On("runAction", sar, suffixMatcher("act/remote-action@v1"), newRemoteAction(sar.Step.Uses)).Return(func(_ context.Context) error { return tt.runError })

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

				cm.On("GetContainerArchive", ctx, "/var/run/act/workflow/pathcmd.txt").Return(io.NopCloser(&bytes.Buffer{}), nil)
			}

			err := sar.pre()(ctx)
			if err == nil {
				err = sar.main()(ctx)
			}

			assert.Equal(t, tt.runError, err)
			assert.Equal(t, tt.mocks.cloned, clonedAction)
			assert.Equal(t, tt.result, sar.RunContext.StepResults["step"])

			sarm.AssertExpectations(t)
			cm.AssertExpectations(t)
		})
	}
}

func TestStepActionRemotePre(t *testing.T) {
	table := []struct {
		name      string
		stepModel *model.Step
	}{
		{
			name: "run-pre",
			stepModel: &model.Step{
				Uses: "org/repo/path@ref",
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			clonedAction := false
			sarm := &stepActionRemoteMocks{}

			origStepAtionRemoteNewCloneExecutor := stepActionRemoteNewCloneExecutor
			stepActionRemoteNewCloneExecutor = func(_ git.NewGitCloneExecutorInput) common.Executor {
				return func(_ context.Context) error {
					clonedAction = true
					return nil
				}
			}
			defer (func() {
				stepActionRemoteNewCloneExecutor = origStepAtionRemoteNewCloneExecutor
			})()

			sar := &stepActionRemote{
				Step: tt.stepModel,
				RunContext: &RunContext{
					Config: &Config{
						GitHubInstance: "https://github.com",
					},
					Run: &model.Run{
						JobID: "1",
						Workflow: &model.Workflow{
							Jobs: map[string]*model.Job{
								"1": {},
							},
						},
					},
				},
				readAction: sarm.readAction,
			}

			suffixMatcher := func(suffix string) interface{} {
				return mock.MatchedBy(func(actionDir string) bool {
					return strings.HasSuffix(actionDir, suffix)
				})
			}

			sarm.On("readAction", sar.Step, suffixMatcher("org-repo-path@ref"), "path", mock.Anything, mock.Anything).Return(&model.Action{}, nil)

			err := sar.pre()(ctx)

			assert.Nil(t, err)
			assert.Equal(t, true, clonedAction)

			sarm.AssertExpectations(t)
		})
	}
}

func TestStepActionRemotePreThroughAction(t *testing.T) {
	table := []struct {
		name      string
		stepModel *model.Step
	}{
		{
			name: "run-pre",
			stepModel: &model.Step{
				Uses: "org/repo/path@ref",
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			clonedAction := false
			sarm := &stepActionRemoteMocks{}

			origStepAtionRemoteNewCloneExecutor := stepActionRemoteNewCloneExecutor
			stepActionRemoteNewCloneExecutor = func(input git.NewGitCloneExecutorInput) common.Executor {
				return func(_ context.Context) error {
					if input.URL == "https://github.com/org/repo" {
						clonedAction = true
					}
					return nil
				}
			}
			defer (func() {
				stepActionRemoteNewCloneExecutor = origStepAtionRemoteNewCloneExecutor
			})()

			sar := &stepActionRemote{
				Step: tt.stepModel,
				RunContext: &RunContext{
					Config: &Config{
						GitHubInstance:                "https://enterprise.github.com",
						ReplaceGheActionWithGithubCom: []string{"org/repo"},
					},
					Run: &model.Run{
						JobID: "1",
						Workflow: &model.Workflow{
							Jobs: map[string]*model.Job{
								"1": {},
							},
						},
					},
				},
				readAction: sarm.readAction,
			}

			suffixMatcher := func(suffix string) interface{} {
				return mock.MatchedBy(func(actionDir string) bool {
					return strings.HasSuffix(actionDir, suffix)
				})
			}

			sarm.On("readAction", sar.Step, suffixMatcher("org-repo-path@ref"), "path", mock.Anything, mock.Anything).Return(&model.Action{}, nil)

			err := sar.pre()(ctx)

			assert.Nil(t, err)
			assert.Equal(t, true, clonedAction)

			sarm.AssertExpectations(t)
		})
	}
}

func TestStepActionRemotePreThroughActionToken(t *testing.T) {
	table := []struct {
		name      string
		stepModel *model.Step
	}{
		{
			name: "run-pre",
			stepModel: &model.Step{
				Uses: "org/repo/path@ref",
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			clonedAction := false
			sarm := &stepActionRemoteMocks{}

			origStepAtionRemoteNewCloneExecutor := stepActionRemoteNewCloneExecutor
			stepActionRemoteNewCloneExecutor = func(input git.NewGitCloneExecutorInput) common.Executor {
				return func(_ context.Context) error {
					if input.URL == "https://github.com/org/repo" && input.Token == "PRIVATE_ACTIONS_TOKEN_ON_GITHUB" {
						clonedAction = true
					}
					return nil
				}
			}
			defer (func() {
				stepActionRemoteNewCloneExecutor = origStepAtionRemoteNewCloneExecutor
			})()

			sar := &stepActionRemote{
				Step: tt.stepModel,
				RunContext: &RunContext{
					Config: &Config{
						GitHubInstance:                     "https://enterprise.github.com",
						ReplaceGheActionWithGithubCom:      []string{"org/repo"},
						ReplaceGheActionTokenWithGithubCom: "PRIVATE_ACTIONS_TOKEN_ON_GITHUB",
					},
					Run: &model.Run{
						JobID: "1",
						Workflow: &model.Workflow{
							Jobs: map[string]*model.Job{
								"1": {},
							},
						},
					},
				},
				readAction: sarm.readAction,
			}

			suffixMatcher := func(suffix string) interface{} {
				return mock.MatchedBy(func(actionDir string) bool {
					return strings.HasSuffix(actionDir, suffix)
				})
			}

			sarm.On("readAction", sar.Step, suffixMatcher("org-repo-path@ref"), "path", mock.Anything, mock.Anything).Return(&model.Action{}, nil)

			err := sar.pre()(ctx)

			assert.Nil(t, err)
			assert.Equal(t, true, clonedAction)

			sarm.AssertExpectations(t)
		})
	}
}

func TestStepActionRemotePost(t *testing.T) {
	table := []struct {
		name               string
		stepModel          *model.Step
		actionModel        *model.Action
		initialStepResults map[string]*model.StepResult
		IntraActionState   map[string]map[string]string
		expectedEnv        map[string]string
		err                error
		mocks              struct {
			env  bool
			exec bool
		}
	}{
		{
			name: "main-success",
			stepModel: &model.Step{
				ID:   "step",
				Uses: "remote/action@v1",
			},
			actionModel: &model.Action{
				Runs: model.ActionRuns{
					Using:  "node16",
					Post:   "post.js",
					PostIf: "always()",
				},
			},
			initialStepResults: map[string]*model.StepResult{
				"step": {
					Conclusion: model.StepStatusSuccess,
					Outcome:    model.StepStatusSuccess,
					Outputs:    map[string]string{},
				},
			},
			IntraActionState: map[string]map[string]string{
				"step": {
					"key": "value",
				},
			},
			expectedEnv: map[string]string{
				"STATE_key": "value",
			},
			mocks: struct {
				env  bool
				exec bool
			}{
				env:  true,
				exec: true,
			},
		},
		{
			name: "main-failed",
			stepModel: &model.Step{
				ID:   "step",
				Uses: "remote/action@v1",
			},
			actionModel: &model.Action{
				Runs: model.ActionRuns{
					Using:  "node16",
					Post:   "post.js",
					PostIf: "always()",
				},
			},
			initialStepResults: map[string]*model.StepResult{
				"step": {
					Conclusion: model.StepStatusFailure,
					Outcome:    model.StepStatusFailure,
					Outputs:    map[string]string{},
				},
			},
			mocks: struct {
				env  bool
				exec bool
			}{
				env:  true,
				exec: true,
			},
		},
		{
			name: "skip-if-failed",
			stepModel: &model.Step{
				ID:   "step",
				Uses: "remote/action@v1",
			},
			actionModel: &model.Action{
				Runs: model.ActionRuns{
					Using:  "node16",
					Post:   "post.js",
					PostIf: "success()",
				},
			},
			initialStepResults: map[string]*model.StepResult{
				"step": {
					Conclusion: model.StepStatusFailure,
					Outcome:    model.StepStatusFailure,
					Outputs:    map[string]string{},
				},
			},
			mocks: struct {
				env  bool
				exec bool
			}{
				env:  true,
				exec: false,
			},
		},
		{
			name: "skip-if-main-skipped",
			stepModel: &model.Step{
				ID:   "step",
				If:   yaml.Node{Value: "failure()"},
				Uses: "remote/action@v1",
			},
			actionModel: &model.Action{
				Runs: model.ActionRuns{
					Using:  "node16",
					Post:   "post.js",
					PostIf: "always()",
				},
			},
			initialStepResults: map[string]*model.StepResult{
				"step": {
					Conclusion: model.StepStatusSkipped,
					Outcome:    model.StepStatusSkipped,
					Outputs:    map[string]string{},
				},
			},
			mocks: struct {
				env  bool
				exec bool
			}{
				env:  false,
				exec: false,
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			cm := &containerMock{}

			sar := &stepActionRemote{
				env: map[string]string{},
				RunContext: &RunContext{
					Config: &Config{
						GitHubInstance: "https://github.com",
					},
					JobContainer: cm,
					Run: &model.Run{
						JobID: "1",
						Workflow: &model.Workflow{
							Jobs: map[string]*model.Job{
								"1": {},
							},
						},
					},
					StepResults:      tt.initialStepResults,
					IntraActionState: tt.IntraActionState,
					nodeToolFullPath: "node",
				},
				Step:   tt.stepModel,
				action: tt.actionModel,
			}
			sar.RunContext.ExprEval = sar.RunContext.NewExpressionEvaluator(ctx)

			if tt.mocks.exec {
				cm.On("Exec", []string{"node", "/var/run/act/actions/remote-action@v1/post.js"}, sar.env, "", "").Return(func(_ context.Context) error { return tt.err })

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

				cm.On("GetContainerArchive", ctx, "/var/run/act/workflow/pathcmd.txt").Return(io.NopCloser(&bytes.Buffer{}), nil)
			}

			err := sar.post()(ctx)

			assert.Equal(t, tt.err, err)
			if tt.expectedEnv != nil {
				for key, value := range tt.expectedEnv {
					assert.Equal(t, value, sar.env[key])
				}
			}
			// Enshure that StepResults is nil in this test
			assert.Equal(t, sar.RunContext.StepResults["post-step"], (*model.StepResult)(nil))
			cm.AssertExpectations(t)
		})
	}
}

func Test_safeFilename(t *testing.T) {
	tests := []struct {
		s    string
		want string
	}{
		{
			s:    "https://test.com/test/",
			want: "https---test.com-test-",
		},
		{
			s:    `<>:"/\|?*`,
			want: "---------",
		},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			assert.Equalf(t, tt.want, safeFilename(tt.s), "safeFilename(%v)", tt.s)
		})
	}
}
