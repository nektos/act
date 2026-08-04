package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	docker_container "github.com/moby/moby/api/types/container"
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
	ActionOfflineMode                  bool                         // when offline, use caching action contents
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
	ConcurrentJobs                     int                          // Number of max concurrent jobs

	concurrencyManager *concurrencyManager // serializes jobs sharing a `concurrency` group, shared with reusable workflow runners
}

func (config *Config) GetConcurrentJobs() int {
	if config.ConcurrentJobs >= 1 {
		return config.ConcurrentJobs
	}

	ncpu := runtime.NumCPU()
	log.Debugf("Detected CPUs: %d", ncpu)
	if ncpu > 1 {
		return ncpu
	}
	return 1
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

// NewPlanExecutor executes a plan as one independent "workflow run" per
// workflow: every workflow runs its own stages serially while different
// workflows run in parallel, so a workflow run can hold its workflow level
// `concurrency` group from its first to its last job and jobs of other
// workflows are not stage barriers for each other. The total number of
// concurrently executing jobs is limited by GetConcurrentJobs.
func (runner *runnerImpl) NewPlanExecutor(plan *model.Plan) common.Executor {
	maxJobNameLen := new(int)

	log.Debugf("Plan Stages: %v", plan.Stages)

	// the global stage index is the dependency depth; `needs` only references
	// jobs of the same workflow, so per workflow the non-empty depths in
	// order preserve the dependency order
	workflowStages := make(map[*model.Workflow][][]*model.Run)
	lastStageIndex := make(map[*model.Workflow]int)
	workflows := make([]*model.Workflow, 0)
	for i := range plan.Stages {
		for _, run := range plan.Stages[i].Runs {
			workflow := run.Workflow
			stages, seen := workflowStages[workflow]
			if !seen {
				workflows = append(workflows, workflow)
			}
			if !seen || lastStageIndex[workflow] != i {
				stages = append(stages, []*model.Run{})
				lastStageIndex[workflow] = i
			}
			stages[len(stages)-1] = append(stages[len(stages)-1], run)
			workflowStages[workflow] = stages
		}
	}

	jobSlots := make(chan struct{}, runner.config.GetConcurrentJobs())
	log.Debugf("PlanExecutor concurrency: %d", runner.config.GetConcurrentJobs())

	runExecutors := make([]common.Executor, 0, len(workflows))
	for _, workflow := range workflows {
		runExecutors = append(runExecutors, runner.newWorkflowRunExecutor(workflow, workflowStages[workflow], jobSlots, maxJobNameLen))
	}

	return common.NewParallelExecutor(len(runExecutors), runExecutors...).Then(handleFailure(plan))
}

// newWorkflowRunExecutor runs the stages of one workflow serially. If the
// workflow declares a `concurrency` group, the whole run holds the group
// from its first to its last job; a run superseded while pending, or
// cancelled by another run's cancel-in-progress, finishes with all its jobs
// marked 'cancelled' without failing the plan.
func (runner *runnerImpl) newWorkflowRunExecutor(workflow *model.Workflow, stages [][]*model.Run, jobSlots chan struct{}, maxJobNameLen *int) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		runState := newWorkflowRunState()
		defer runState.complete()

		spec, ok, err := runner.workflowConcurrencySpec(ctx, stages)
		if err != nil {
			return err
		}
		if ok {
			if isConcurrencyGroupHeld(ctx, spec.group) {
				// a calling workflow already holds this group, acquiring it
				// again would deadlock on our own caller
				logger.Debugf("Concurrency group '%s' is already held by a calling workflow", spec.group)
			} else {
				release, err := runner.config.concurrency().acquire(ctx, nil, spec, runState.cancelRun)
				if err != nil {
					if isConcurrencyCancellation(err) && ctx.Err() == nil {
						logger.Infof("\U0001F6AB  Workflow run '%s' was cancelled by concurrency group '%s' before it started", workflow.Name, spec.group)
						markRunCancelled(stages)
						return nil
					}
					return err
				}
				defer release()
				ctx = withHeldConcurrencyGroup(ctx, spec.group)
			}
		}

		var firstErr error
		for _, stageRuns := range stages {
			if err := runner.newStageExecutor(stageRuns, runState, jobSlots, maxJobNameLen)(ctx); err != nil {
				firstErr = err
				break
			}
		}
		runState.complete()
		if runState.cancelled.Load() && ctx.Err() == nil {
			// a run cancelled by cancel-in-progress of another run does not
			// fail the plan, its jobs carry the result 'cancelled'
			return nil
		}
		return firstErr
	}
}

// workflowConcurrencySpec evaluates the workflow level `concurrency`
// settings; only the github, inputs and vars contexts are available
func (runner *runnerImpl) workflowConcurrencySpec(ctx context.Context, stages [][]*model.Run) (concurrencySpec, bool, error) {
	var firstRun *model.Run
	for _, stageRuns := range stages {
		if len(stageRuns) > 0 {
			firstRun = stageRuns[0]
			break
		}
	}
	if firstRun == nil || firstRun.Workflow.Concurrency() == nil {
		return concurrencySpec{}, false, nil
	}
	rc := runner.newRunContext(ctx, firstRun, nil)
	return evaluateConcurrency(ctx, rc.NewExpressionEvaluator(ctx), firstRun.Workflow.Concurrency())
}

func markRunCancelled(stages [][]*model.Run) {
	for _, stageRuns := range stages {
		for _, run := range stageRuns {
			run.Job().Result = "cancelled"
		}
	}
}

func (runner *runnerImpl) newStageExecutor(stageRuns []*model.Run, runState *workflowRunState, jobSlots chan struct{}, maxJobNameLen *int) common.Executor {
	return func(ctx context.Context) error {
		pipeline := make([]common.Executor, 0)
		for _, run := range stageRuns {
			log.Debugf("Stages Runs: %v", stageRuns)
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
				rc := runner.newRunContext(ctx, run, matrix)
				rc.JobName = rc.Name
				rc.runState = runState
				rc.jobSlots = jobSlots
				if len(matrixes) > 1 {
					rc.Name = fmt.Sprintf("%s-%d", rc.Name, i+1)
				}
				if len(rc.String()) > *maxJobNameLen {
					*maxJobNameLen = len(rc.String())
				}
				stageExecutor = append(stageExecutor, func(ctx context.Context) error {
					jobName := fmt.Sprintf("%-*s", *maxJobNameLen, rc.String())
					executor, err := rc.Executor()

					if err != nil {
						return err
					}

					// the job slot limiting how many jobs run at once is
					// taken inside the executor, after the job acquired its
					// concurrency group, so queued jobs do not occupy slots
					return executor(common.WithJobErrorContainer(WithJobLogger(ctx, rc.Run.JobID, jobName, rc.Config, &rc.Masks, matrix)))
				})
			}
			pipeline = append(pipeline, common.NewParallelExecutor(maxParallel, stageExecutor...))
		}

		return common.NewParallelExecutor(len(pipeline), pipeline...)(ctx)
	}
}

func handleFailure(plan *model.Plan) common.Executor {
	return func(_ context.Context) error {
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
