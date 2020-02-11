package runner

import (
	"context"
	"io/ioutil"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

// Runner provides capabilities to run GitHub actions
type Runner interface {
	NewPlanExecutor(plan *model.Plan) common.Executor
	NewRunExecutor(run *model.Run) common.Executor
}

// Config contains the config for a new runner
type Config struct {
	Workdir         string // path to working directory
	EventName       string // name of event to run
	EventPath       string // path to JSON file to use for event.json in containers
	ReuseContainers bool   // reuse containers to maintain state
	ForcePull       bool   // force pulling of the image, if already present
}

type runnerImpl struct {
	config    *Config
	eventJSON string
}

// New Creates a new Runner
func New(runnerConfig *Config) (Runner, error) {
	runner := &runnerImpl{
		config: runnerConfig,
	}

	runner.eventJSON = "{}"
	if runnerConfig.EventPath != "" {
		log.Debugf("Reading event.json from %s", runner.config.EventPath)
		eventJSONBytes, err := ioutil.ReadFile(runner.config.EventPath)
		if err != nil {
			return nil, err
		}
		runner.eventJSON = string(eventJSONBytes)
	}
	return runner, nil
}

func (runner *runnerImpl) NewPlanExecutor(plan *model.Plan) common.Executor {
	pipeline := make([]common.Executor, 0)
	for _, stage := range plan.Stages {
		stageExecutor := make([]common.Executor, 0)
		for _, run := range stage.Runs {
			stageExecutor = append(stageExecutor, runner.NewRunExecutor(run))
		}
		pipeline = append(pipeline, common.NewParallelExecutor(stageExecutor...))
	}

	return common.NewPipelineExecutor(pipeline...)
}

func (runner *runnerImpl) NewRunExecutor(run *model.Run) common.Executor {
	rc := new(RunContext)
	rc.Config = runner.config
	rc.Run = run
	rc.EventJSON = runner.eventJSON
	return func(ctx context.Context) error {
		ctx = WithJobLogger(ctx, rc.Run.String())
		return rc.Executor()(ctx)
	}
}
