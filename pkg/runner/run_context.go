package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Name         string
	Config       *Config
	Matrix       map[string]interface{}
	Run          *model.Run
	EventJSON    string
	Env          map[string]string
	ExtraPath    []string
	CurrentStep  string
	StepResults  map[string]*stepResult
	ExprEval     ExpressionEvaluator
	JobContainer container.Container
}

func (rc *RunContext) String() string {
	return fmt.Sprintf("%s/%s", rc.Run.Workflow.Name, rc.Name)
}

type stepResult struct {
	Success bool              `json:"success"`
	Outputs map[string]string `json:"outputs"`
}

// GetEnv returns the env for the context
func (rc *RunContext) GetEnv() map[string]string {
	if rc.Env == nil {
		rc.Env = mergeMaps(rc.Config.Env, rc.Run.Workflow.Env, rc.Run.Job().Env)
	}
	return rc.Env
}

func (rc *RunContext) jobContainerName() string {
	return createContainerName("act", rc.String())
}

func (rc *RunContext) startJobHostContainer() common.Executor {
	return func(ctx context.Context) error {
		rc.JobContainer = container.NewHost(&container.NewHostContainerInput{
			WorkingDir: rc.Config.Workdir,
		})

		return common.NewPipelineExecutor(
			rc.JobContainer.Create(),
		)(ctx)
	}
}

func (rc *RunContext) startJobContainer() common.Executor {
	image, err := rc.platformImage()

	if err != nil {
		log.Errorf("\U0001F6A7  Unable to run on missing image for platform '%+v'", rc.Run.Job().Name)
	}

	return func(ctx context.Context) error {
		rawLogger := common.Logger(ctx).WithField("raw_output", true)
		logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) bool {
			if rc.Config.LogOutput {
				rawLogger.Infof(s)
			} else {
				rawLogger.Debugf(s)
			}
			return true
		})

		common.Logger(ctx).Infof("\U0001f680  Start image=%s", image)
		name := rc.jobContainerName()

		envList := make([]string, 0)
		bindModifiers := ""
		if runtime.GOOS == "darwin" {
			bindModifiers = ":delegated"
		}

		envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TOOL_CACHE", "/opt/hostedtoolcache"))
		envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_OS", "Linux"))
		envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TEMP", "/tmp"))

		binds := []string{
			fmt.Sprintf("%s:%s", "/var/run/docker.sock", "/var/run/docker.sock"),
		}
		if rc.Config.BindWorkdir {
			binds = append(binds, fmt.Sprintf("%s:%s%s", rc.Config.Workdir, "/github/workspace", bindModifiers))
		}

		rc.JobContainer = container.NewContainer(&container.NewContainerInput{
			Cmd:        nil,
			Entrypoint: []string{"/usr/bin/tail", "-f", "/dev/null"},
			WorkingDir: "/github/workspace",
			Image:      image,
			Name:       name,
			Env:        envList,
			Mounts: map[string]string{
				name:            "/github",
				"act-toolcache": "/toolcache",
				"act-actions":   "/actions",
			},
			NetworkMode: "host",
			Binds:       binds,
			Stdout:      logWriter,
			Stderr:      logWriter,
		})

		var copyWorkspace bool
		var copyToPath string
		if !rc.Config.BindWorkdir {
			copyToPath, copyWorkspace = rc.localCheckoutPath()
			copyToPath = filepath.Join("/github/workspace", copyToPath)
		}

		return common.NewPipelineExecutor(
			rc.JobContainer.Pull(rc.Config.ForcePull),
			rc.JobContainer.Remove().IfBool(!rc.Config.ReuseContainers),
			rc.JobContainer.Create(),
			rc.JobContainer.Start(false),
			rc.JobContainer.CopyDir(copyToPath, rc.Config.Workdir+"/.").IfBool(copyWorkspace),
			rc.JobContainer.Copy("/github/", &container.FileEntry{
				Name: "workflow/event.json",
				Mode: 644,
				Body: rc.EventJSON,
			}, &container.FileEntry{
				Name: "home/.act",
				Mode: 644,
				Body: "",
			}),
		)(ctx)
	}
}
func (rc *RunContext) execJobContainer(cmd []string, env map[string]string) common.Executor {
	return func(ctx context.Context) error {
		return rc.JobContainer.Exec(cmd, env)(ctx)
	}
}
func (rc *RunContext) stopJobContainer() common.Executor {
	return func(ctx context.Context) error {
		if rc.JobContainer != nil && !rc.Config.ReuseContainers {
			return rc.JobContainer.Remove().
				Then(container.NewDockerVolumeRemoveExecutor(rc.jobContainerName(), false))(ctx)
		}
		return nil
	}
}

// ActionCacheDir is for rc
func (rc *RunContext) ActionCacheDir() string {
	var xdgCache string
	var ok bool
	if xdgCache, ok = os.LookupEnv("XDG_CACHE_HOME"); !ok {
		if home, ok := os.LookupEnv("HOME"); ok {
			xdgCache = fmt.Sprintf("%s/.cache", home)
		}
	}
	return filepath.Join(xdgCache, "act")
}

// Executor returns a pipeline executor for all the steps in the job
func (rc *RunContext) Executor() common.Executor {
	steps := make([]common.Executor, 0)

	steps = append(steps, func(ctx context.Context) error {
		if len(rc.Matrix) > 0 {
			common.Logger(ctx).Infof("\U0001F9EA  Matrix: %v", rc.Matrix)
		}
		return nil
	})

	platform := rc.extractPlatform()

	if platform.Engine == model.PlatformEngineDocker {
		steps = append(steps, rc.startJobContainer())
	} else if platform.Engine == model.PlatformEngineHost {
		steps = append(steps, rc.startJobHostContainer())
	}

	for i, step := range rc.Run.Job().Steps {
		if step.ID == "" {
			step.ID = fmt.Sprintf("%d", i)
		}
		steps = append(steps, rc.newStepExecutor(step))
	}

	if platform.Engine == model.PlatformEngineDocker {
		steps = append(steps, rc.stopJobContainer())
	}

	return common.NewPipelineExecutor(steps...).If(rc.isEnabled)
}

func (rc *RunContext) newStepExecutor(step *model.Step) common.Executor {
	sc := &StepContext{
		RunContext: rc,
		Step:       step,
	}
	return func(ctx context.Context) error {
		rc.CurrentStep = sc.Step.ID
		rc.StepResults[rc.CurrentStep] = &stepResult{
			Success: true,
			Outputs: make(map[string]string),
		}

		_ = sc.setupEnv()(ctx)
		rc.ExprEval = sc.NewExpressionEvaluator()

		if !rc.EvalBool(sc.Step.If) {
			log.Debugf("Skipping step '%s' due to '%s'", sc.Step.String(), sc.Step.If)
			return nil
		}

		common.Logger(ctx).Infof("\u2B50  Run %s", sc.Step)
		err := sc.Executor()(ctx)
		if err == nil {
			common.Logger(ctx).Infof("  \u2705  Success - %s", sc.Step)
		} else {
			common.Logger(ctx).Errorf("  \u274C  Failure - %s", sc.Step)
			rc.StepResults[rc.CurrentStep].Success = false
		}
		return err
	}
}

func (rc *RunContext) runsOnContainer() bool {
	platform := rc.extractPlatform()

	if platform.Engine == model.PlatformEngineDocker {
		return true
	}

	return false
}

func (rc *RunContext) runsOnHost() bool {
	platform := rc.extractPlatform()

	if platform.Engine == model.PlatformEngineHost {
		return true
	}

	return false
}

func (rc *RunContext) extractRunsOn() string {
	job := rc.Run.Job()

	runsOn := job.RunsOn()

	if len(runsOn) > 0 {
		log.Infof("\U0001F6A7  Multiple 'runs-on' detected: will run only on: '%+v'", runsOn[0])
	}

	return runsOn[0]
}

func (rc *RunContext) extractPlatform() *model.Platform {
	runsOn := rc.extractRunsOn()

	platformName := rc.ExprEval.Interpolate(runsOn)
	platform := rc.Config.Platforms[strings.ToLower(platformName)]

	return platform
}

func (rc *RunContext) platformImage() (string, error) {
	job := rc.Run.Job()

	c := job.Container()
	if c != nil {
		return c.Image, nil
	}

	platform := rc.extractPlatform()

	if platform.Supported == false {
		return "", nil
	}

	image := platform.Image

	if image != "" {
		return image, nil
	}

	return "", nil
}

func (rc *RunContext) isEnabled(ctx context.Context) bool {
	job := rc.Run.Job()
	log := common.Logger(ctx)
	if !rc.EvalBool(job.If) {
		log.Debugf("Skipping job '%s' due to '%s'", job.Name, job.If)
		return false
	}

	if rc.runsOnHost() {
		return true
	}

	_, err := rc.platformImage()

	if err != nil {
		log.Infof("\U0001F6A7  Unable to run on missing image for platform '%+v'", job.RunsOn())
		return false
	}

	return true
}

// EvalBool evaluates an expression against current run context
func (rc *RunContext) EvalBool(expr string) bool {
	if expr != "" {
		//v, err := rc.ExprEval.Evaluate(fmt.Sprintf("if (%s) { true } else { false }", expr))
		expr := fmt.Sprintf("Boolean(%s)", expr)
		v, err := rc.ExprEval.Evaluate(expr)
		if err != nil {
			log.Errorf("Error evaluating expression '%s' - %v", expr, err)
			return false
		}
		log.Debugf("expression '%s' evaluated to '%s'", expr, v)
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

func createContainerName(parts ...string) string {
	name := make([]string, 0)
	pattern := regexp.MustCompile("[^a-zA-Z0-9]")
	partLen := (30 / len(parts)) - 1
	for i, part := range parts {
		if i == len(parts)-1 {
			name = append(name, pattern.ReplaceAllString(part, "-"))
		} else {
			name = append(name, trimToLen(pattern.ReplaceAllString(part, "-"), partLen))
		}
	}
	return strings.Trim(strings.Join(name, "-"), "-")
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
	token, ok := rc.Config.Secrets["GITHUB_TOKEN"]
	if !ok {
		token = os.Getenv("GITHUB_TOKEN")
	}

	ghc := &githubContext{
		Event:     make(map[string]interface{}),
		EventPath: "/github/workflow/event.json",
		Workflow:  rc.Run.Workflow.Name,
		RunID:     "1",
		RunNumber: "1",
		Actor:     "nektos/act",
		EventName: rc.Config.EventName,
		Token:     token,
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

	if ghc.EventName == "pull_request" {
		ghc.BaseRef = asString(nestedMapLookup(ghc.Event, "pull_request", "base", "ref"))
		ghc.HeadRef = asString(nestedMapLookup(ghc.Event, "pull_request", "head", "ref"))
	}

	return ghc
}

func (ghc *githubContext) isLocalCheckout(step *model.Step) bool {
	if step.Type() != model.StepTypeUsesActionRemote {
		return false
	}
	remoteAction := newRemoteAction(step.Uses)
	if !remoteAction.IsCheckout() {
		return false
	}

	if repository, ok := step.With["repository"]; ok && repository != ghc.Repository {
		return false
	}
	if repository, ok := step.With["ref"]; ok && repository != ghc.Ref {
		return false
	}
	return true
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	} else if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func nestedMapLookup(m map[string]interface{}, ks ...string) (rval interface{}) {
	var ok bool

	if len(ks) == 0 { // degenerate input
		return nil
	}
	if rval, ok = m[ks[0]]; !ok {
		return nil
	} else if len(ks) == 1 { // we've reached the final key
		return rval
	} else if m, ok = rval.(map[string]interface{}); !ok {
		return nil
	} else { // 1+ more keys
		return nestedMapLookup(m, ks[1:]...)
	}
}

func (rc *RunContext) withGithubEnv(env map[string]string) map[string]string {
	github := rc.getGithubContext()
	env["HOME"] = "/github/home"
	env["GITHUB_WORKFLOW"] = github.Workflow
	env["GITHUB_RUN_ID"] = github.RunID
	env["GITHUB_RUN_NUMBER"] = github.RunNumber
	env["GITHUB_ACTION"] = github.Action
	env["GITHUB_ACTIONS"] = "true"
	env["GITHUB_ACTOR"] = github.Actor
	env["GITHUB_REPOSITORY"] = github.Repository
	env["GITHUB_EVENT_NAME"] = github.EventName
	env["GITHUB_EVENT_PATH"] = github.EventPath
	env["GITHUB_WORKSPACE"] = github.Workspace
	env["GITHUB_SHA"] = github.Sha
	env["GITHUB_REF"] = github.Ref
	env["GITHUB_TOKEN"] = github.Token
	return env
}

func (rc *RunContext) localCheckoutPath() (string, bool) {
	ghContext := rc.getGithubContext()
	for _, step := range rc.Run.Job().Steps {
		if ghContext.isLocalCheckout(step) {
			return step.With["path"], true
		}
	}
	return "", false
}
