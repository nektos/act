package actions

import (
	"context"
	"io"
)

// Runner provides capabilities to run GitHub actions
type Runner interface {
	EventGrapher
	EventLister
	EventRunner
	ActionRunner
	io.Closer
}

// EventGrapher to list the actions
type EventGrapher interface {
	GraphEvent(eventName string) ([][]string, error)
}

// EventLister to list the events
type EventLister interface {
	ListEvents() []string
}

// EventRunner to run the actions for a given event
type EventRunner interface {
	RunEvent() error
}

// ActionRunner to run a specific actions
type ActionRunner interface {
	RunActions(actionNames ...string) error
}

// RunnerConfig contains the config for a new runner
type RunnerConfig struct {
	Ctx          context.Context // context to use for the run
	Dryrun       bool            // don't start any of the containers
	WorkingDir   string          // base directory to use
	WorkflowPath string          // path to load main.workflow file, relative to WorkingDir
	EventName    string          // name of event to run
	EventPath    string          // path to JSON file to use for event.json in containers, relative to WorkingDir
}

type environmentApplier interface {
	applyEnvironment(map[string]string)
}
