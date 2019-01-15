package actions

import (
	"context"
)

// Workflows provides capabilities to work with the workflow file
type Workflows interface {
	EventGrapher
	EventLister
	ActionRunner
	EventRunner
	Close()
}

// EventGrapher to list the actions
type EventGrapher interface {
	GraphEvent(eventName string) ([][]string, error)
}

// EventLister to list the events
type EventLister interface {
	ListEvents() []string
}

// ActionRunner to run an action
type ActionRunner interface {
	RunAction(ctx context.Context, dryrun bool, action string) error
}

// EventRunner to run an event
type EventRunner interface {
	RunEvent(ctx context.Context, dryrun bool, event string) error
}

type workflowDef struct {
	On       string
	Resolves []string
}

type actionDef struct {
	Needs   []string
	Uses    string
	Runs    []string
	Args    []string
	Env     map[string]string
	Secrets []string
}

type workflowsFile struct {
	TempDir      string
	WorkingDir   string
	WorkflowPath string
	Workflow     map[string]workflowDef
	Action       map[string]actionDef
}
