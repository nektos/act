package runner

import (
	"context"

	"github.com/nektos/act/pkg/common"
)

type ActionProvider interface {
	SetupAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor
	RunAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor
	ExecuteNode12Action(sc *StepContext, containerActionDir string, ctx context.Context, maybeCopyToActionDir func() error) error
	ExecuteNode12PostAction(sc *StepContext, containerActionDir string, ctx context.Context) error
}

type actProvider struct{}

func (a *actProvider) ExecuteNode12Action(sc *StepContext, containerActionDir string, ctx context.Context, maybeCopyToActionDir func() error) error {
	return nil
}

func (a *actProvider) ExecuteNode12PostAction(sc *StepContext, containerActionDir string, ctx context.Context) error {
	return nil
}

func (a *actProvider) SetupAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func (a *actProvider) RunAction(sc *StepContext, actionDir string, actionPath string, localAction bool) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}

func NewActionProvider() ActionProvider {
	return &actProvider{}
}
