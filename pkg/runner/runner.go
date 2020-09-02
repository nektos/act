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
	Actor           string            // the user that triggered the event
	Workdir         string            // path to working directory
	BindWorkdir     bool              // bind the workdir to the job container
	EventName       string            // name of event to run
	EventPath       string            // path to JSON file to use for event.json in containers
	DefaultBranch   string            // name of the main branch for this repository
	ReuseContainers bool              // reuse containers to maintain state
	ForcePull       bool              // force pulling of the image, if already present
	LogOutput       bool              // log the output from docker run
	Env             map[string]string // env for containers
	Secrets         map[string]string // list of secrets
	Platforms       map[string]string // list of platforms
	Privileged      bool              // use privileged mode
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
	maxJobNameLen := 0

	pipeline := make([]common.Executor, 0)
	for _, stage := range plan.Stages {
		stageExecutor := make([]common.Executor, 0)
		for _, run := range stage.Runs {
			job := run.Job()
			matrixes := job.GetMatrixes()

			for i, matrix := range matrixes {
				rc := runner.newRunContext(run, matrix)
				if len(matrixes) > 1 {
					rc.Name = fmt.Sprintf("%s-%d", rc.Name, i+1)
				}
				if len(rc.String()) > maxJobNameLen {
					maxJobNameLen = len(rc.String())
				}
				stageExecutor = append(stageExecutor, func(ctx context.Context) error {
					jobName := fmt.Sprintf("%-*s", maxJobNameLen, rc.String())
					return rc.Executor()(WithJobLogger(ctx, jobName, rc.Config.Secrets))
				})
			}
		}
		pipeline = append(pipeline, common.NewParallelExecutor(stageExecutor...))
	}

	return common.NewPipelineExecutor(pipeline...)
}

func (runner *runnerImpl) newRunContext(run *model.Run, matrix map[string]interface{}) *RunContext {
	rc := &RunContext{
		Config:      runner.config,
		Run:         run,
		EventJSON:   runner.eventJSON,
		StepResults: make(map[string]*stepResult),
		Matrix:      matrix,
	}
	rc.ExprEval = rc.NewExpressionEvaluator()
	rc.Name = rc.ExprEval.Interpolate(run.String())
	return rc
}
