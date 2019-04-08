package actions

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/actions/workflow-parser/model"
	"github.com/actions/workflow-parser/parser"
	"github.com/nektos/act/common"
	log "github.com/sirupsen/logrus"
)

type runnerImpl struct {
	config         *RunnerConfig
	workflowConfig *model.Configuration
	tempDir        string
	eventJSON      string
}

// NewRunner Creates a new Runner
func NewRunner(runnerConfig *RunnerConfig) (Runner, error) {
	runner := &runnerImpl{
		config: runnerConfig,
	}

	init := common.NewPipelineExecutor(
		runner.setupTempDir,
		runner.setupWorkingDir,
		runner.setupWorkflows,
		runner.setupEvent,
	)

	return runner, init()
}

func (runner *runnerImpl) setupTempDir() error {
	var err error
	runner.tempDir, err = ioutil.TempDir("", "act-")
	return err
}

func (runner *runnerImpl) setupWorkingDir() error {
	var err error
	runner.config.WorkingDir, err = filepath.Abs(runner.config.WorkingDir)
	log.Debugf("Setting working dir to %s", runner.config.WorkingDir)
	return err
}

func (runner *runnerImpl) setupWorkflows() error {
	runner.config.WorkflowPath = runner.resolvePath(runner.config.WorkflowPath)
	log.Debugf("Loading workflow config from %s", runner.config.WorkflowPath)
	workflowReader, err := os.Open(runner.config.WorkflowPath)
	if err != nil {
		return err
	}
	defer workflowReader.Close()

	runner.workflowConfig, err = parser.Parse(workflowReader)
	return err
}

func (runner *runnerImpl) setupEvent() error {
	runner.eventJSON = "{}"
	if runner.config.EventPath != "" {
		runner.config.EventPath = runner.resolvePath(runner.config.EventPath)
		log.Debugf("Reading event.json from %s", runner.config.EventPath)
		eventJSONBytes, err := ioutil.ReadFile(runner.config.EventPath)
		if err != nil {
			return err
		}
		runner.eventJSON = string(eventJSONBytes)
	}
	return nil
}

func (runner *runnerImpl) resolvePath(path string) string {
	if path == "" {
		return path
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(runner.config.WorkingDir, path)
	}
	return path
}

// ListEvents gets all the events in the workflows file
func (runner *runnerImpl) ListEvents() []string {
	log.Debugf("Listing all events")
	events := make([]string, 0)
	for _, w := range runner.workflowConfig.Workflows {
		events = append(events, w.On)
	}

	// sort the list based on depth of dependencies
	sort.Slice(events, func(i, j int) bool {
		return events[i] < events[j]
	})

	return events
}

// GraphEvent builds an execution path
func (runner *runnerImpl) GraphEvent(eventName string) ([][]string, error) {
	log.Debugf("Listing actions for event '%s'", eventName)
	resolves := runner.resolveEvent(eventName)
	return newExecutionGraph(runner.workflowConfig, resolves...), nil
}

// RunAction runs a set of actions in parallel, and their dependencies
func (runner *runnerImpl) RunActions(actionNames ...string) error {
	log.Debugf("Running actions %+q", actionNames)
	graph := newExecutionGraph(runner.workflowConfig, actionNames...)

	pipeline := make([]common.Executor, 0)
	for _, actions := range graph {
		stage := make([]common.Executor, 0)
		for _, actionName := range actions {
			stage = append(stage, runner.newActionExecutor(actionName))
		}
		pipeline = append(pipeline, common.NewParallelExecutor(stage...))
	}

	executor := common.NewPipelineExecutor(pipeline...)
	return executor()
}

// RunEvent runs the actions for a single event
func (runner *runnerImpl) RunEvent() error {
	log.Debugf("Running event '%s'", runner.config.EventName)
	resolves := runner.resolveEvent(runner.config.EventName)
	log.Debugf("Running actions %s -> %s", runner.config.EventName, resolves)
	return runner.RunActions(resolves...)
}

func (runner *runnerImpl) Close() error {
	return os.RemoveAll(runner.tempDir)
}

// get list of resolves for an event
func (runner *runnerImpl) resolveEvent(eventName string) []string {
	workflows := runner.workflowConfig.GetWorkflows(eventName)
	resolves := make([]string, 0)
	for _, workflow := range workflows {
		for _, resolve := range workflow.Resolves {
			found := false
			for _, r := range resolves {
				if r == resolve {
					found = true
					break
				}
			}
			if !found {
				resolves = append(resolves, resolve)
			}
		}
	}
	return resolves
}
