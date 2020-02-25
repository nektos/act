package runner

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

// Runner provides capabilities to run GitHub actions
type Runner interface {
	NewPlanExecutor(plan *model.Plan) common.Executor
}

// Config contains the config for a new runner
type Config struct {
	Workdir         string            // path to working directory
	BindWorkdir     bool              // bind the workdir to the job container
	EventName       string            // name of event to run
	EventPath       string            // path to JSON file to use for event.json in containers
	ReuseContainers bool              // reuse containers to maintain state
	ForcePull       bool              // force pulling of the image, if already present
	LogOutput       bool              // log the output from docker run
	Secrets         map[string]string // list of secrets
	Platforms       map[string]string // list of platforms
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
	maxJobNameLen := plan.MaxRunNameLen()

	pipeline := make([]common.Executor, 0)
	for _, stage := range plan.Stages {
		stageExecutor := make([]common.Executor, 0)
		for _, run := range stage.Runs {
			job := run.Job()
			matrixes := job.GetMatrixes()

			jobName := fmt.Sprintf("%-*s", maxJobNameLen, run.String())
			for _, matrix := range matrixes {
				m := matrix
				runExecutor := runner.newRunExecutor(run, matrix)
				stageExecutor = append(stageExecutor, func(ctx context.Context) error {
					ctx = WithJobLogger(ctx, jobName)
					if len(m) > 0 {
						common.Logger(ctx).Infof("\U0001F9EA  Matrix: %v", m)
					}
					return runExecutor(ctx)
				})
			}
		}
		pipeline = append(pipeline, common.NewParallelExecutor(stageExecutor...))
	}

	return common.NewPipelineExecutor(pipeline...)
}

func (runner *runnerImpl) newRunExecutor(run *model.Run, matrix map[string]interface{}) common.Executor {
	rc := &RunContext{
		Config:      runner.config,
		Run:         run,
		EventJSON:   runner.eventJSON,
		StepResults: make(map[string]*stepResult),
		Matrix:      matrix,
	}
	rc.ExprEval = rc.NewExpressionEvaluator()
	return rc.Executor()
}
