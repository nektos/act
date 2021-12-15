package runner

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/common/executor"
	"github.com/nektos/act/pkg/common/logger"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner/config"

	log "github.com/sirupsen/logrus"
)

// Runner provides capabilities to run GitHub actions
type Runner interface {
	NewPlanExecutor(plan *model.Plan) executor.Executor
}

type runnerImpl struct {
	config    *config.Config
	eventJSON string
}

// New Creates a new Runner
func New(runnerConfig *config.Config) (Runner, error) {
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

// NewPlanExecutor ...
//nolint:gocyclo
func (runner *runnerImpl) NewPlanExecutor(plan *model.Plan) executor.Executor {
	maxJobNameLen := 0

	stagePipeline := make([]executor.Executor, 0)
	for i := range plan.Stages {
		s := i
		stage := plan.Stages[i]
		stagePipeline = append(stagePipeline, func(ctx context.Context) error {
			pipeline := make([]executor.Executor, 0)
			for r, run := range stage.Runs {
				stageExecutor := make([]executor.Executor, 0)
				job := run.Job()

				if job.Uses != "" {
					return fmt.Errorf("reusable workflows are currently not supported (see https://github.com/nektos/act/issues/826 for updates)")
				}

				if job.Strategy != nil {
					strategyRc := runner.newRunContext(run, nil)
					if err := strategyRc.NewExpressionEvaluator().EvaluateYamlNode(&job.Strategy.RawMatrix); err != nil {
						log.Errorf("Error while evaluating matrix: %v", err)
					}
				}
				matrixes := job.GetMatrixes()
				maxParallel := 4
				if job.Strategy != nil {
					maxParallel = job.Strategy.MaxParallel
				}

				if len(matrixes) < maxParallel {
					maxParallel = len(matrixes)
				}

				for i, matrix := range matrixes {
					rc := runner.newRunContext(run, matrix)
					rc.JobName = rc.Name
					if len(matrixes) > 1 {
						rc.Name = fmt.Sprintf("%s-%d", rc.Name, i+1)
					}
					if len(rc.String()) > maxJobNameLen {
						maxJobNameLen = len(rc.String())
					}
					stageExecutor = append(stageExecutor, func(ctx context.Context) error {
						jobName := fmt.Sprintf("%-*s", maxJobNameLen, rc.String())
						return rc.Executor().Finally(func(ctx context.Context) error {
							isLastRunningContainer := func(currentStage int, currentRun int) bool {
								return currentStage == len(plan.Stages)-1 && currentRun == len(stage.Runs)-1
							}

							if runner.config.AutoRemove && isLastRunningContainer(s, r) {
								log.Infof("Cleaning up container for job %s", rc.JobName)
								if err := rc.stopJobContainer()(ctx); err != nil {
									log.Errorf("Error while cleaning container: %v", err)
								}
							}

							return nil
						})(common.WithJobErrorContainer(logger.WithJobLogger(ctx, jobName, rc.Config, &rc.Masks)))
					})
				}
				pipeline = append(pipeline, executor.NewParallelExecutor(maxParallel, stageExecutor...))
			}
			var ncpu int
			info, err := container.GetHostInfo(ctx)
			if err != nil {
				log.Errorf("failed to obtain container engine info: %s", err)
				ncpu = 1 // sane default?
			} else {
				ncpu = info.NCPU
			}
			return executor.NewParallelExecutor(ncpu, pipeline...)(ctx)
		})
	}

	return executor.NewPipelineExecutor(stagePipeline...).Then(handleFailure(plan))
}

func handleFailure(plan *model.Plan) executor.Executor {
	return func(ctx context.Context) error {
		for _, stage := range plan.Stages {
			for _, run := range stage.Runs {
				if run.Job().Result == "failure" {
					return fmt.Errorf("Job '%s' failed", run.String())
				}
			}
		}
		return nil
	}
}

func (runner *runnerImpl) newRunContext(run *model.Run, matrix map[string]interface{}) *RunContext {
	rc := &RunContext{
		Config:      runner.config,
		Run:         run,
		EventJSON:   runner.eventJSON,
		StepResults: make(map[string]*model.StepResult),
		Matrix:      matrix,
	}
	rc.ExprEval = rc.NewExpressionEvaluator()
	rc.Name = rc.ExprEval.Interpolate(run.String())
	return rc
}
