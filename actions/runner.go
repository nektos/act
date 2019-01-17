package actions

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/nektos/act/common"
	log "github.com/sirupsen/logrus"
)

type runnerImpl struct {
	config    *RunnerConfig
	workflows *workflowsFile
	tempDir   string
	eventJSON string
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

	runner.workflows, err = parseWorkflowsFile(workflowReader)
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
	for _, w := range runner.workflows.Workflow {
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
	workflow, _, err := runner.workflows.getWorkflow(eventName)
	if err != nil {
		return nil, err
	}
	return runner.workflows.newExecutionGraph(workflow.Resolves...), nil
}

// RunAction runs a set of actions in parallel, and their dependencies
func (runner *runnerImpl) RunActions(actionNames ...string) error {
	log.Debugf("Running actions %+q", actionNames)
	graph := runner.workflows.newExecutionGraph(actionNames...)

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
	workflow, _, err := runner.workflows.getWorkflow(runner.config.EventName)
	if err != nil {
		return err
	}

	log.Debugf("Running actions %s -> %s", runner.config.EventName, workflow.Resolves)
	return runner.RunActions(workflow.Resolves...)
}

func (runner *runnerImpl) Close() error {
	return os.RemoveAll(runner.tempDir)
}
