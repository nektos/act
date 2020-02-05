package runner

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

// Runner provides capabilities to run GitHub actions
type Runner interface {
	PlanRunner
	io.Closer
}

// PlanRunner to run a specific actions
type PlanRunner interface {
	RunPlan(plan *model.Plan) error
}

// Config contains the config for a new runner
type Config struct {
	Dryrun          bool   // don't start any of the containers
	EventName       string // name of event to run
	EventPath       string // path to JSON file to use for event.json in containers
	ReuseContainers bool   // reuse containers to maintain state
	ForcePull       bool   // force pulling of the image, if already present
}

type runnerImpl struct {
	config    *Config
	tempDir   string
	eventJSON string
}

// NewRunner Creates a new Runner
func NewRunner(runnerConfig *Config) (Runner, error) {
	runner := &runnerImpl{
		config: runnerConfig,
	}

	init := common.NewPipelineExecutor(
		runner.setupTempDir,
		runner.setupEvent,
	)

	return runner, init()
}

func (runner *runnerImpl) setupTempDir() error {
	var err error
	runner.tempDir, err = ioutil.TempDir("", "act-")
	return err
}

func (runner *runnerImpl) setupEvent() error {
	runner.eventJSON = "{}"
	if runner.config.EventPath != "" {
		log.Debugf("Reading event.json from %s", runner.config.EventPath)
		eventJSONBytes, err := ioutil.ReadFile(runner.config.EventPath)
		if err != nil {
			return err
		}
		runner.eventJSON = string(eventJSONBytes)
	}
	return nil
}

func (runner *runnerImpl) RunPlan(plan *model.Plan) error {
	pipeline := make([]common.Executor, 0)
	for _, stage := range plan.Stages {
		stageExecutor := make([]common.Executor, 0)
		for _, run := range stage.Runs {
			stageExecutor = append(stageExecutor, runner.newRunExecutor(run))
		}
		pipeline = append(pipeline, common.NewParallelExecutor(stageExecutor...))
	}

	executor := common.NewPipelineExecutor(pipeline...)
	return executor()
}

func (runner *runnerImpl) Close() error {
	return os.RemoveAll(runner.tempDir)
}
