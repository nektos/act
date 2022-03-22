package runner

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
)

// Runner provides capabilities to run GitHub actions
type Runner interface {
	NewPlanExecutor(plan *model.Plan) common.Executor
}

// Config contains the config for a new runner
type Config struct {
	Actor                 string                       // the user that triggered the event
	Workdir               string                       // path to working directory
	BindWorkdir           bool                         // bind the workdir to the job container
	EventName             string                       // name of event to run
	EventPath             string                       // path to JSON file to use for event.json in containers
	DefaultBranch         string                       // name of the main branch for this repository
	ReuseContainers       bool                         // reuse containers to maintain state
	ForcePull             bool                         // force pulling of the image, even if already present
	ForceRebuild          bool                         // force rebuilding local docker image action
	LogOutput             bool                         // log the output from docker run
	JSONLogger            bool                         // use json or text logger
	Env                   map[string]string            // env for containers
	Secrets               map[string]string            // list of secrets
	InsecureSecrets       bool                         // switch hiding output when printing to terminal
	Platforms             map[string]string            // list of platforms
	Privileged            bool                         // use privileged mode
	UsernsMode            string                       // user namespace to use
	ContainerArchitecture string                       // Desired OS/architecture platform for running containers
	ContainerDaemonSocket string                       // Path to Docker daemon socket
	UseGitIgnore          bool                         // controls if paths in .gitignore should not be copied into container, default true
	GitHubInstance        string                       // GitHub instance to use, default "github.com"
	ContainerCapAdd       []string                     // list of kernel capabilities to add to the containers
	ContainerCapDrop      []string                     // list of kernel capabilities to remove from the containers
	AutoRemove            bool                         // controls if the container is automatically removed upon workflow completion
	ArtifactServerPath    string                       // the path where the artifact server stores uploads
	ArtifactServerPort    string                       // the port the artifact server binds to
	CompositeRestrictions *model.CompositeRestrictions // describes which features are available in composite actions
	NoSkipCheckout        bool                         // do not skip actions/checkout
}

// Resolves the equivalent host path inside the container
// This is required for windows and WSL 2 to translate things like C:\Users\Myproject to /mnt/users/Myproject
// For use in docker volumes and binds
func (config *Config) containerPath(path string) string {
	if runtime.GOOS == "windows" && strings.Contains(path, "/") {
		log.Error("You cannot specify linux style local paths (/mnt/etc) on Windows as it does not understand them.")
		return ""
	}

	abspath, err := filepath.Abs(path)
	if err != nil {
		log.Error(err)
		return ""
	}

	// Test if the path is a windows path
	windowsPathRegex := regexp.MustCompile(`^([a-zA-Z]):\\(.+)$`)
	windowsPathComponents := windowsPathRegex.FindStringSubmatch(abspath)

	// Return as-is if no match
	if windowsPathComponents == nil {
		return abspath
	}

	// Convert to WSL2-compatible path if it is a windows path
	// NOTE: Cannot use filepath because it will use the wrong path separators assuming we want the path to be windows
	// based if running on Windows, and because we are feeding this to Docker, GoLang auto-path-translate doesn't work.
	driveLetter := strings.ToLower(windowsPathComponents[1])
	translatedPath := strings.ReplaceAll(windowsPathComponents[2], `\`, `/`)
	// Should make something like /mnt/c/Users/person/My Folder/MyActProject
	result := strings.Join([]string{"/mnt", driveLetter, translatedPath}, `/`)
	return result
}

// Resolves the equivalent host path inside the container
// This is required for windows and WSL 2 to translate things like C:\Users\Myproject to /mnt/users/Myproject
func (config *Config) ContainerWorkdir() string {
	return config.containerPath(config.Workdir)
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

	stagePipeline := make([]common.Executor, 0)
	for i := range plan.Stages {
		s := i
		stage := plan.Stages[i]
		stagePipeline = append(stagePipeline, func(ctx context.Context) error {
			pipeline := make([]common.Executor, 0)
			for r, run := range stage.Runs {
				stageExecutor := make([]common.Executor, 0)
				job := run.Job()
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
						})(common.WithJobErrorContainer(WithJobLogger(ctx, jobName, rc.Config, &rc.Masks)))
					})
				}
				pipeline = append(pipeline, common.NewParallelExecutor(maxParallel, stageExecutor...))
			}
			var ncpu int
			info, err := container.GetHostInfo(ctx)
			if err != nil {
				log.Errorf("failed to obtain container engine info: %s", err)
				ncpu = 1 // sane default?
			} else {
				ncpu = info.NCPU
			}
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
