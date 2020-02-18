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
	NewRunExecutor(run *model.Run, matrix map[string]interface{}) common.Executor
}

// Config contains the config for a new runner
type Config struct {
	Workdir         string            // path to working directory
	EventName       string            // name of event to run
	EventPath       string            // path to JSON file to use for event.json in containers
	ReuseContainers bool              // reuse containers to maintain state
	ForcePull       bool              // force pulling of the image, if already present
	LogOutput       bool              // log the output from docker run
	Secrets         map[string]string // list of secrets
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
			matrixes := make([]map[string]interface{}, 0)
			if job.Strategy != nil {
				includes := make([]map[string]interface{}, 0)
				for _, v := range job.Strategy.Matrix["include"] {
					includes = append(includes, v.(map[string]interface{}))
				}
				delete(job.Strategy.Matrix, "include")

				excludes := make([]map[string]interface{}, 0)
				for _, v := range job.Strategy.Matrix["exclude"] {
					excludes = append(excludes, v.(map[string]interface{}))
				}
				delete(job.Strategy.Matrix, "exclude")

				matrixProduct := common.CartesianProduct(job.Strategy.Matrix)

			MATRIX:
				for _, matrix := range matrixProduct {
					for _, exclude := range excludes {
						if commonKeysMatch(matrix, exclude) {
							log.Debugf("Skipping matrix '%v' due to exclude '%v'", matrix, exclude)
							continue MATRIX
						}
					}
					for _, include := range includes {
						if commonKeysMatch(matrix, include) {
							log.Debugf("Setting add'l values on matrix '%v' due to include '%v'", matrix, include)
							for k, v := range include {
								matrix[k] = v
							}
						}
					}
					matrixes = append(matrixes, matrix)
				}

			} else {
				matrixes = append(matrixes, make(map[string]interface{}))
			}

			jobName := fmt.Sprintf("%-*s", maxJobNameLen, run.String())
			for _, matrix := range matrixes {
				m := matrix
				runExecutor := runner.NewRunExecutor(run, matrix)
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

func commonKeysMatch(a map[string]interface{}, b map[string]interface{}) bool {
	for aKey, aVal := range a {
		if bVal, ok := b[aKey]; ok && aVal != bVal {
			return false
		}
	}
	return true
}

func (runner *runnerImpl) NewRunExecutor(run *model.Run, matrix map[string]interface{}) common.Executor {
	rc := new(RunContext)
	rc.Config = runner.config
	rc.Run = run
	rc.EventJSON = runner.eventJSON
	rc.StepResults = make(map[string]*stepResult)
	rc.Matrix = matrix
	rc.ExprEval = rc.NewExpressionEvaluator()
	return rc.Executor()
}
