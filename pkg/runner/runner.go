package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	docker_container "github.com/docker/docker/api/types/container"
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
	Actor                              string                       // the user that triggered the event
	Workdir                            string                       // path to working directory
	ActionCacheDir                     string                       // path used for caching action contents
	BindWorkdir                        bool                         // bind the workdir to the job container
	EventName                          string                       // name of event to run
	EventPath                          string                       // path to JSON file to use for event.json in containers
	DefaultBranch                      string                       // name of the main branch for this repository
	ReuseContainers                    bool                         // reuse containers to maintain state
	ForcePull                          bool                         // force pulling of the image, even if already present
	ForceRebuild                       bool                         // force rebuilding local docker image action
	LogOutput                          bool                         // log the output from docker run
	JSONLogger                         bool                         // use json or text logger
	LogPrefixJobID                     bool                         // switches from the full job name to the job id
	Env                                map[string]string            // env for containers
	Inputs                             map[string]string            // manually passed action inputs
	Secrets                            map[string]string            // list of secrets
	Vars                               map[string]string            // list of vars
	Token                              string                       // GitHub token
	InsecureSecrets                    bool                         // switch hiding output when printing to terminal
	Platforms                          map[string]string            // list of platforms
	Privileged                         bool                         // use privileged mode
	UsernsMode                         string                       // user namespace to use
	ContainerArchitecture              string                       // Desired OS/architecture platform for running containers
	ContainerDaemonSocket              string                       // Path to Docker daemon socket
	ContainerOptions                   string                       // Options for the job container
	UseGitIgnore                       bool                         // controls if paths in .gitignore should not be copied into container, default true
	GitHubInstance                     string                       // GitHub instance to use, default "github.com"
	ContainerCapAdd                    []string                     // list of kernel capabilities to add to the containers
	ContainerCapDrop                   []string                     // list of kernel capabilities to remove from the containers
	AutoRemove                         bool                         // controls if the container is automatically removed upon workflow completion
	ArtifactServerPath                 string                       // the path where the artifact server stores uploads
	ArtifactServerAddr                 string                       // the address the artifact server binds to
	ArtifactServerPort                 string                       // the port the artifact server binds to
	NoSkipCheckout                     bool                         // do not skip actions/checkout
	RemoteName                         string                       // remote name in local git repo config
	ReplaceGheActionWithGithubCom      []string                     // Use actions from GitHub Enterprise instance to GitHub
	ReplaceGheActionTokenWithGithubCom string                       // Token of private action repo on GitHub.
	Matrix                             map[string]map[string]bool   // Matrix config to run
	ContainerNetworkMode               docker_container.NetworkMode // the network mode of job containers (the value of --network)
	ActionCache                        ActionCache                  // Use a custom ActionCache Implementation
}

type caller struct {
	runContext *RunContext
}

type runnerImpl struct {
	config    *Config
	eventJSON string
	caller    *caller // the job calling this runner (caller of a reusable workflow)
}

// New Creates a new Runner
func New(runnerConfig *Config) (Runner, error) {
	runner := &runnerImpl{
		config: runnerConfig,
	}

	return runner.configure()
}

func (runner *runnerImpl) configure() (Runner, error) {
	runner.eventJSON = "{}"
	if runner.config.EventPath != "" {
		log.Debugf("Reading event.json from %s", runner.config.EventPath)
		eventJSONBytes, err := os.ReadFile(runner.config.EventPath)
		if err != nil {
			return nil, err
		}
		runner.eventJSON = string(eventJSONBytes)
	} else if len(runner.config.Inputs) != 0 {
		eventMap := map[string]map[string]string{
			"inputs": runner.config.Inputs,
		}
		eventJSON, err := json.Marshal(eventMap)
		if err != nil {
			return nil, err
		}
		runner.eventJSON = string(eventJSON)
	}
	return runner, nil
}

// NewPlanExecutor ...
func (runner *runnerImpl) NewPlanExecutor(plan *model.Plan) common.Executor {
	maxJobNameLen := 0

	stagePipeline := make([]common.Executor, 0)
	log.Debugf("Plan Stages: %v", plan.Stages)

	for i := range plan.Stages {
		stage := plan.Stages[i]
		stagePipeline = append(stagePipeline, func(ctx context.Context) error {
			pipeline := make([]common.Executor, 0)
			for _, run := range stage.Runs {
				log.Debugf("Stages Runs: %v", stage.Runs)
				stageExecutor := make([]common.Executor, 0)
				job := run.Job()
				log.Debugf("Job.Name: %v", job.Name)
				log.Debugf("Job.RawNeeds: %v", job.RawNeeds)
				log.Debugf("Job.RawRunsOn: %v", job.RawRunsOn)
				log.Debugf("Job.Env: %v", job.Env)
				log.Debugf("Job.If: %v", job.If)
				for step := range job.Steps {
					if nil != job.Steps[step] {
						log.Debugf("Job.Steps: %v", job.Steps[step].String())
					}
				}
				log.Debugf("Job.TimeoutMinutes: %v", job.TimeoutMinutes)
				log.Debugf("Job.Services: %v", job.Services)
				log.Debugf("Job.Strategy: %v", job.Strategy)
				log.Debugf("Job.RawContainer: %v", job.RawContainer)
				log.Debugf("Job.Defaults.Run.Shell: %v", job.Defaults.Run.Shell)
				log.Debugf("Job.Defaults.Run.WorkingDirectory: %v", job.Defaults.Run.WorkingDirectory)
				log.Debugf("Job.Outputs: %v", job.Outputs)
				log.Debugf("Job.Uses: %v", job.Uses)
				log.Debugf("Job.With: %v", job.With)
				// log.Debugf("Job.RawSecrets: %v", job.RawSecrets)
				log.Debugf("Job.Result: %v", job.Result)

				if job.Strategy != nil {
					log.Debugf("Job.Strategy.FailFast: %v", job.Strategy.FailFast)
					log.Debugf("Job.Strategy.MaxParallel: %v", job.Strategy.MaxParallel)
					log.Debugf("Job.Strategy.FailFastString: %v", job.Strategy.FailFastString)
					log.Debugf("Job.Strategy.MaxParallelString: %v", job.Strategy.MaxParallelString)
					log.Debugf("Job.Strategy.RawMatrix: %v", job.Strategy.RawMatrix)

					strategyRc := runner.newRunContext(ctx, run, nil)
					if err := strategyRc.NewExpressionEvaluator(ctx).EvaluateYamlNode(ctx, &job.Strategy.RawMatrix); err != nil {
						log.Errorf("Error while evaluating matrix: %v", err)
					}
				}

				var matrixes []map[string]interface{}
				if m, err := job.GetMatrixes(); err != nil {
					log.Errorf("Error while get job's matrix: %v", err)
				} else {
					log.Debugf("Job Matrices: %v", m)
					log.Debugf("Runner Matrices: %v", runner.config.Matrix)
					matrixes = selectMatrixes(m, runner.config.Matrix)
				}
				log.Debugf("Final matrix after applying user inclusions '%v'", matrixes)

				maxParallel := 4
				if job.Strategy != nil {
					maxParallel = job.Strategy.MaxParallel
				}

				if len(matrixes) < maxParallel {
					maxParallel = len(matrixes)
				}

				for i, matrix := range matrixes {
					matrix := matrix
					rc := runner.newRunContext(ctx, run, matrix)
					rc.JobName = rc.Name
					if len(matrixes) > 1 {
						rc.Name = fmt.Sprintf("%s-%d", rc.Name, i+1)
					}
					if len(rc.String()) > maxJobNameLen {
						maxJobNameLen = len(rc.String())
					}
					stageExecutor = append(stageExecutor, func(ctx context.Context) error {
						jobName := fmt.Sprintf("%-*s", maxJobNameLen, rc.String())
						executor, err := rc.Executor()

						if err != nil {
							return err
						}

						return executor(common.WithJobErrorContainer(WithJobLogger(ctx, rc.Run.JobID, jobName, rc.Config, &rc.Masks, matrix)))
					})
				}
				pipeline = append(pipeline, common.NewParallelExecutor(maxParallel, stageExecutor...))
			}
			ncpu := runtime.NumCPU()
			if 1 > ncpu {
				ncpu = 1
			}
			log.Debugf("Detected CPUs: %d", ncpu)
			return common.NewParallelExecutor(ncpu, pipeline...)(ctx)
		})
	}

	return common.NewPipelineExecutor(stagePipeline...).Then(handleFailure(plan))
}

func handleFailure(plan *model.Plan) common.Executor {
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

func selectMatrixes(originalMatrixes []map[string]interface{}, targetMatrixValues map[string]map[string]bool) []map[string]interface{} {
	matrixes := make([]map[string]interface{}, 0)
	for _, original := range originalMatrixes {
		flag := true
		for key, val := range original {
			if allowedVals, ok := targetMatrixValues[key]; ok {
				valToString := fmt.Sprintf("%v", val)
				if _, ok := allowedVals[valToString]; !ok {
					flag = false
				}
			}
		}
		if flag {
			matrixes = append(matrixes, original)
		}
	}
	return matrixes
}

func (runner *runnerImpl) newRunContext(ctx context.Context, run *model.Run, matrix map[string]interface{}) *RunContext {
	rc := &RunContext{
		Config:      runner.config,
		Run:         run,
		EventJSON:   runner.eventJSON,
		StepResults: make(map[string]*model.StepResult),
		Matrix:      matrix,
		caller:      runner.caller,
	}
	rc.ExprEval = rc.NewExpressionEvaluator(ctx)
	rc.Name = rc.ExprEval.Interpolate(ctx, run.String())

	return rc
}
