package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/nektos/act/pkg/container"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

// RunContext contains info about current job
type RunContext struct {
	Config      *Config
	Matrix      map[string]interface{}
	Run         *model.Run
	EventJSON   string
	Env         map[string]string
	Tempdir     string
	ExtraPath   []string
	CurrentStep string
	StepResults map[string]*stepResult
	ExprEval    ExpressionEvaluator
}

type stepResult struct {
	Success bool              `json:"success"`
	Outputs map[string]string `json:"outputs"`
}

// GetEnv returns the env for the context
func (rc *RunContext) GetEnv() map[string]string {
	if rc.Env == nil {
		rc.Env = mergeMaps(rc.Run.Workflow.Env, rc.Run.Job().Env)
	}
	return rc.Env
}

// Close cleans up temp dir
func (rc *RunContext) Close(ctx context.Context) error {
	return os.RemoveAll(rc.Tempdir)
}

// Executor returns a pipeline executor for all the steps in the job
func (rc *RunContext) Executor() common.Executor {

	err := rc.setupTempDir()
	if err != nil {
		return common.NewErrorExecutor(err)
	}
	steps := make([]common.Executor, 0)

	for i, step := range rc.Run.Job().Steps {
		if step.ID == "" {
			step.ID = fmt.Sprintf("%d", i)
		}
		s := step
		steps = append(steps, func(ctx context.Context) error {
			rc.CurrentStep = s.ID
			rc.StepResults[rc.CurrentStep] = &stepResult{
				Success: true,
				Outputs: make(map[string]string),
			}
			rc.ExprEval = rc.NewStepExpressionEvaluator(s)

			if !rc.EvalBool(s.If) {
				log.Debugf("Skipping step '%s' due to '%s'", s.String(), s.If)
				return nil
			}

			common.Logger(ctx).Infof("\u2B50  Run %s", s)
			err := rc.newStepExecutor(s)(ctx)
			if err == nil {
				common.Logger(ctx).Infof("  \u2705  Success - %s", s)
			} else {
				common.Logger(ctx).Errorf("  \u274C  Failure - %s", s)
				rc.StepResults[rc.CurrentStep].Success = false
			}
			return err
		})
	}
	return func(ctx context.Context) error {
		defer rc.Close(ctx)
		job := rc.Run.Job()
		log := common.Logger(ctx)
		if !rc.EvalBool(job.If) {
			log.Debugf("Skipping job '%s' due to '%s'", job.Name, job.If)
			return nil
		}

		platformName := rc.ExprEval.Interpolate(rc.Run.Job().RunsOn)
		if img, ok := rc.Config.Platforms[strings.ToLower(platformName)]; !ok || img == "" {
			log.Infof("  \U0001F6A7  Skipping unsupported platform '%s'", platformName)
			return nil
		}

		nullLogger := logrus.New()
		nullLogger.Out = ioutil.Discard
		if !rc.Config.ReuseContainers {
			rc.newContainerCleaner()(common.WithLogger(ctx, nullLogger))
		}

		err := common.NewPipelineExecutor(steps...)(ctx)

		if !rc.Config.ReuseContainers {
			rc.newContainerCleaner()(common.WithLogger(ctx, nullLogger))
		}

		return err
	}
}

// EvalBool evaluates an expression against current run context
func (rc *RunContext) EvalBool(expr string) bool {
	if expr != "" {
		v, err := rc.ExprEval.Evaluate(expr)
		if err != nil {
			log.Errorf("Error evaluating expression '%s' - %v", expr, err)
			return false
		}
		return v == "true"
	}
	return true
}

func mergeMaps(maps ...map[string]string) map[string]string {
	rtnMap := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			rtnMap[k] = v
		}
	}
	return rtnMap
}

func (rc *RunContext) setupTempDir() error {
	var err error
	tempBase := ""
	if runtime.GOOS == "darwin" {
		tempBase = "/tmp"
	}
	rc.Tempdir, err = ioutil.TempDir(tempBase, "act-")
	if err != nil {
		return err
	}
	err = os.Chmod(rc.Tempdir, 0755)
	if err != nil {
		return err
	}
	log.Debugf("Setup tempdir %s", rc.Tempdir)
	return err
}

func (rc *RunContext) pullImage(containerSpec *model.ContainerSpec) common.Executor {
	return func(ctx context.Context) error {
		return container.NewDockerPullExecutor(container.NewDockerPullExecutorInput{
			Image:     containerSpec.Image,
			ForcePull: rc.Config.ForcePull,
		})(ctx)
	}
}

func (rc *RunContext) runContainer(containerSpec *model.ContainerSpec) common.Executor {
	return func(ctx context.Context) error {
		ghReader, err := rc.createGithubTarball()
		if err != nil {
			return err
		}

		envList := make([]string, 0)
		for k, v := range containerSpec.Env {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}
		var cmd, entrypoint []string
		if containerSpec.Args != "" {
			cmd = strings.Fields(rc.ExprEval.Interpolate(containerSpec.Args))
		}
		if containerSpec.Entrypoint != "" {
			entrypoint = strings.Fields(rc.ExprEval.Interpolate(containerSpec.Entrypoint))
		}

		rawLogger := common.Logger(ctx).WithField("raw_output", true)
		logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) {
			if rc.Config.LogOutput {
				rawLogger.Infof(s)
			} else {
				rawLogger.Debugf(s)
			}
		})

		return container.NewDockerRunExecutor(container.NewDockerRunExecutorInput{
			Cmd:        cmd,
			Entrypoint: entrypoint,
			Image:      containerSpec.Image,
			WorkingDir: "/github/workspace",
			Env:        envList,
			Name:       containerSpec.Name,
			Binds: []string{
				fmt.Sprintf("%s:%s", rc.Config.Workdir, "/github/workspace"),
				fmt.Sprintf("%s:%s", rc.Tempdir, "/github/home"),
				fmt.Sprintf("%s:%s", "/var/run/docker.sock", "/var/run/docker.sock"),
			},
			Content:         map[string]io.Reader{"/github": ghReader},
			ReuseContainers: containerSpec.Reuse,
			Stdout:          logWriter,
			Stderr:          logWriter,
		})(ctx)
	}
}

func (rc *RunContext) createGithubTarball() (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var files = []struct {
		Name string
		Mode int64
		Body string
	}{
		{"workflow/event.json", 0644, rc.EventJSON},
	}
	for _, file := range files {
		log.Debugf("Writing entry to tarball %s len:%d", file.Name, len(rc.EventJSON))
		hdr := &tar.Header{
			Name: file.Name,
			Mode: file.Mode,
			Size: int64(len(rc.EventJSON)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(rc.EventJSON)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil

}

func (rc *RunContext) createContainerName() string {
	containerName := rc.Run.String()
	containerName = regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(containerName, "-")

	prefix := ""
	suffix := ""
	containerName = trimToLen(containerName, 30-(len(prefix)+len(suffix)))
	return fmt.Sprintf("%s%s%s", prefix, containerName, suffix)

}

func (rc *RunContext) createStepContainerName(stepID string) string {

	prefix := regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(rc.createContainerName(), "-")
	suffix := regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(stepID, "-")
	containerName := trimToLen(prefix, 30-len(suffix))
	return fmt.Sprintf("%s%s%s", prefix, containerName, suffix)
}

func trimToLen(s string, l int) string {
	if l < 0 {
		l = 0
	}
	if len(s) > l {
		return s[:l]
	}
	return s
}

type jobContext struct {
	Status    string `json:"status"`
	Container struct {
		ID      string `json:"id"`
		Network string `json:"network"`
	} `json:"container"`
	Services map[string]struct {
		ID string `json:"id"`
	} `json:"services"`
}

func (rc *RunContext) getJobContext() *jobContext {
	jobStatus := "success"
	for _, stepStatus := range rc.StepResults {
		if !stepStatus.Success {
			jobStatus = "failure"
			break
		}
	}
	return &jobContext{
		Status: jobStatus,
	}
}

func (rc *RunContext) getStepsContext() map[string]*stepResult {
	return rc.StepResults
}

type githubContext struct {
	Event      map[string]interface{} `json:"event"`
	EventPath  string                 `json:"event_path"`
	Workflow   string                 `json:"workflow"`
	RunID      string                 `json:"run_id"`
	RunNumber  string                 `json:"run_number"`
	Actor      string                 `json:"actor"`
	Repository string                 `json:"repository"`
	EventName  string                 `json:"event_name"`
	Sha        string                 `json:"sha"`
	Ref        string                 `json:"ref"`
	HeadRef    string                 `json:"head_ref"`
	BaseRef    string                 `json:"base_ref"`
	Token      string                 `json:"token"`
	Workspace  string                 `json:"workspace"`
	Action     string                 `json:"action"`
}

func (rc *RunContext) getGithubContext() *githubContext {
	ghc := &githubContext{
		Event:     make(map[string]interface{}),
		EventPath: "/github/workflow/event.json",
		Workflow:  rc.Run.Workflow.Name,
		RunID:     "1",
		RunNumber: "1",
		Actor:     "nektos/act",

		EventName: rc.Config.EventName,
		Token:     os.Getenv("GITHUB_TOKEN"),
		Workspace: "/github/workspace",
		Action:    rc.CurrentStep,
	}

	repoPath := rc.Config.Workdir
	repo, err := common.FindGithubRepo(repoPath)
	if err != nil {
		log.Warningf("unable to get git repo: %v", err)
	} else {
		ghc.Repository = repo
	}

	_, sha, err := common.FindGitRevision(repoPath)
	if err != nil {
		log.Warningf("unable to get git revision: %v", err)
	} else {
		ghc.Sha = sha
	}

	ref, err := common.FindGitRef(repoPath)
	if err != nil {
		log.Warningf("unable to get git ref: %v", err)
	} else {
		log.Debugf("using github ref: %s", ref)
		ghc.Ref = ref
	}
	err = json.Unmarshal([]byte(rc.EventJSON), &ghc.Event)
	if err != nil {
		logrus.Error(err)
	}
	return ghc
}

func (rc *RunContext) withGithubEnv(env map[string]string) map[string]string {
	github := rc.getGithubContext()
	env["HOME"] = "/github/home"
	env["GITHUB_WORKFLOW"] = github.Workflow
	env["GITHUB_RUN_ID"] = github.RunID
	env["GITHUB_RUN_NUMBER"] = github.RunNumber
	env["GITHUB_ACTION"] = github.Action
	env["GITHUB_ACTOR"] = github.Actor
	env["GITHUB_REPOSITORY"] = github.Repository
	env["GITHUB_EVENT_NAME"] = github.EventName
	env["GITHUB_EVENT_PATH"] = github.EventPath
	env["GITHUB_WORKSPACE"] = github.Workspace
	env["GITHUB_SHA"] = github.Sha
	env["GITHUB_REF"] = github.Ref
	return env
}
