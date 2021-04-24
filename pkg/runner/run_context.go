package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
)

// RunContext contains info about current job
type RunContext struct {
	Name           string
	Config         *Config
	Matrix         map[string]interface{}
	Run            *model.Run
	EventJSON      string
	Env            map[string]string
	ExtraPath      []string
	CurrentStep    string
	StepResults    map[string]*stepResult
	ExprEval       ExpressionEvaluator
	JobContainer   container.Container
	OutputMappings map[MappableOutput]MappableOutput
}

type MappableOutput struct {
	StepID     string
	OutputName string
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
	rc.Env["ACT"] = "true"
	return rc.Env
}

func (rc *RunContext) jobContainerName() string {
	return createContainerName("act", rc.String())
}

func (rc *RunContext) startJobContainer() common.Executor {
	image := rc.platformImage()

	return func(ctx context.Context) error {
		rawLogger := common.Logger(ctx).WithField("raw_output", true)
		logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) bool {
			if rc.Config.LogOutput {
				rawLogger.Infof("%s", s)
			} else {
				rawLogger.Debugf("%s", s)
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
			binds = append(binds, fmt.Sprintf("%s:%s%s", rc.Config.Workdir, rc.Config.Workdir, bindModifiers))
		}

		if rc.Config.ContainerArchitecture == "" {
			rc.Config.ContainerArchitecture = fmt.Sprintf("%s/%s", "linux", runtime.GOARCH)
		}

		rc.JobContainer = container.NewContainer(&container.NewContainerInput{
			Cmd:        nil,
			Entrypoint: []string{"/usr/bin/tail", "-f", "/dev/null"},
			WorkingDir: rc.Config.Workdir,
			Image:      image,
			Name:       name,
			Env:        envList,
			Mounts: map[string]string{
				name:            filepath.Dir(rc.Config.Workdir),
				"act-toolcache": "/toolcache",
				"act-actions":   "/actions",
			},
			NetworkMode: "host",
			Binds:       binds,
			Stdout:      logWriter,
			Stderr:      logWriter,
			Privileged:  rc.Config.Privileged,
			UsernsMode:  rc.Config.UsernsMode,
			Platform:    rc.Config.ContainerArchitecture,
		})

		var copyWorkspace bool
		var copyToPath string
		if !rc.Config.BindWorkdir {
			copyToPath, copyWorkspace = rc.localCheckoutPath()
			copyToPath = filepath.Join(rc.Config.Workdir, copyToPath)
		}

		return common.NewPipelineExecutor(
			rc.JobContainer.Pull(rc.Config.ForcePull),
			rc.stopJobContainer(),
			rc.JobContainer.Create(),
			rc.JobContainer.Start(false),
			rc.JobContainer.CopyDir(copyToPath, rc.Config.Workdir+string(filepath.Separator)+".").IfBool(copyWorkspace),
			rc.JobContainer.Copy(filepath.Dir(rc.Config.Workdir), &container.FileEntry{
				Name: "workflow/event.json",
				Mode: 0644,
				Body: rc.EventJSON,
			}, &container.FileEntry{
				Name: "workflow/envs.txt",
				Mode: 0644,
				Body: "",
			}, &container.FileEntry{
				Name: "home/.act",
				Mode: 0644,
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

// stopJobContainer removes the job container (if it exists) and its volume (if it exists) if !rc.Config.ReuseContainers
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
	if xdgCache, ok = os.LookupEnv("XDG_CACHE_HOME"); !ok || xdgCache == "" {
		if home, err := homedir.Dir(); err == nil {
			xdgCache = filepath.Join(home, ".cache")
		} else if xdgCache, err = filepath.Abs("."); err != nil {
			log.Fatal(err)
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

	steps = append(steps, rc.startJobContainer())

	for i, step := range rc.Run.Job().Steps {
		if step.ID == "" {
			step.ID = fmt.Sprintf("%d", i)
		}
		steps = append(steps, rc.newStepExecutor(step))
	}
	steps = append(steps, rc.stopJobContainer())

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

		exprEval, err := sc.setupEnv(ctx)
		if err != nil {
			return err
		}
		rc.ExprEval = exprEval

		runStep, err := rc.EvalBool(sc.Step.If)
		if err != nil {
			common.Logger(ctx).Errorf("  \u274C  Error in if: expression - %s", sc.Step)
			rc.StepResults[rc.CurrentStep].Success = false
			return err
		}
		if !runStep {
			log.Debugf("Skipping step '%s' due to '%s'", sc.Step.String(), sc.Step.If)
			return nil
		}

		common.Logger(ctx).Infof("\u2B50  Run %s", sc.Step)
		err = sc.Executor()(ctx)
		if err == nil {
			common.Logger(ctx).Infof("  \u2705  Success - %s", sc.Step)
		} else {
			common.Logger(ctx).Errorf("  \u274C  Failure - %s", sc.Step)

			if sc.Step.ContinueOnError {
				common.Logger(ctx).Infof("Failed but continue next step")
				err = nil
				rc.StepResults[rc.CurrentStep].Success = true
			} else {
				rc.StepResults[rc.CurrentStep].Success = false
			}
		}
		return err
	}
}

func (rc *RunContext) platformImage() string {
	job := rc.Run.Job()

	c := job.Container()
	if c != nil {
		return rc.ExprEval.Interpolate(c.Image)
	}

	if job.RunsOn() == nil {
		log.Errorf("'runs-on' key not defined in %s", rc.String())
	}

	for _, runnerLabel := range job.RunsOn() {
		platformName := rc.ExprEval.Interpolate(runnerLabel)
		image := rc.Config.Platforms[strings.ToLower(platformName)]
		if image != "" {
			return image
		}
	}

	return ""
}

func (rc *RunContext) isEnabled(ctx context.Context) bool {
	job := rc.Run.Job()
	l := common.Logger(ctx)
	runJob, err := rc.EvalBool(job.If)
	if err != nil {
		common.Logger(ctx).Errorf("  \u274C  Error in if: expression - %s", job.Name)
		return false
	}
	if !runJob {
		l.Debugf("Skipping job '%s' due to '%s'", job.Name, job.If)
		return false
	}

	img := rc.platformImage()
	if img == "" {
		if job.RunsOn() == nil {
			log.Errorf("'runs-on' key not defined in %s", rc.String())
		}

		for _, runnerLabel := range job.RunsOn() {
			platformName := rc.ExprEval.Interpolate(runnerLabel)
			l.Infof("\U0001F6A7  Skipping unsupported platform '%+v'", platformName)
		}
		return false
	}
	return true
}

var splitPattern *regexp.Regexp

// EvalBool evaluates an expression against current run context
func (rc *RunContext) EvalBool(expr string) (bool, error) {
	if splitPattern == nil {
		splitPattern = regexp.MustCompile(fmt.Sprintf(`%s|%s|\S+`, expressionPattern.String(), operatorPattern.String()))
	}
	if strings.HasPrefix(strings.TrimSpace(expr), "!") {
		return false, errors.New("expressions starting with ! must be wrapped in ${{ }}")
	}
	if expr != "" {
		parts := splitPattern.FindAllString(expr, -1)
		var evaluatedParts []string
		for i, part := range parts {
			if operatorPattern.MatchString(part) {
				evaluatedParts = append(evaluatedParts, part)
				continue
			}

			interpolatedPart, isString := rc.ExprEval.InterpolateWithStringCheck(part)

			// This peculiar transformation has to be done because the GitHub parser
			// treats false returned from contexts as a string, not a boolean.
			// Hence env.SOMETHING will be evaluated to true in an if: expression
			// regardless if SOMETHING is set to false, true or any other string.
			// It also handles some other weirdness that I found by trial and error.
			if (expressionPattern.MatchString(part) && // it is an expression
				!strings.Contains(part, "!")) && // but it's not negated
				interpolatedPart == "false" && // and the interpolated string is false
				(isString || previousOrNextPartIsAnOperator(i, parts)) { // and it's of type string or has an logical operator before or after
				interpolatedPart = fmt.Sprintf("'%s'", interpolatedPart) // then we have to quote the false expression
			}

			evaluatedParts = append(evaluatedParts, interpolatedPart)
		}

		joined := strings.Join(evaluatedParts, " ")
		v, _, err := rc.ExprEval.Evaluate(fmt.Sprintf("Boolean(%s)", joined))
		if err != nil {
			return false, err
		}
		log.Debugf("expression '%s' evaluated to '%s'", expr, v)
		return v == "true", nil
	}
	return true, nil
}

func previousOrNextPartIsAnOperator(i int, parts []string) bool {
	operator := false
	if i > 0 {
		operator = operatorPattern.MatchString(parts[i-1])
	}
	if i+1 < len(parts) {
		operator = operator || operatorPattern.MatchString(parts[i+1])
	}
	return operator
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
			// If any part has a '-<number>' on the end it is likely part of a matrix job.
			// Let's preserve the number to prevent clashes in container names.
			re := regexp.MustCompile("-[0-9]+$")
			num := re.FindStringSubmatch(part)
			if len(num) > 0 {
				name = append(name, trimToLen(pattern.ReplaceAllString(part, "-"), partLen-len(num[0])))
				name = append(name, num[0])
			} else {
				name = append(name, trimToLen(pattern.ReplaceAllString(part, "-"), partLen))
			}
		}
	}
	return strings.ReplaceAll(strings.Trim(strings.Join(name, "-"), "-"), "--", "-")
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
	runID := rc.Config.Env["GITHUB_RUN_ID"]
	if runID == "" {
		runID = "1"
	}
	runNumber := rc.Config.Env["GITHUB_RUN_NUMBER"]
	if runNumber == "" {
		runNumber = "1"
	}
	ghc := &githubContext{
		Event:     make(map[string]interface{}),
		EventPath: fmt.Sprintf("%s/%s", filepath.Dir(rc.Config.Workdir), "workflow/event.json"),
		Workflow:  rc.Run.Workflow.Name,
		RunID:     runID,
		RunNumber: runNumber,
		Actor:     rc.Config.Actor,
		EventName: rc.Config.EventName,
		Token:     token,
		Workspace: rc.Config.Workdir,
		Action:    rc.CurrentStep,
	}

	// Backwards compatibility for configs that require
	// a default rather than being run as a cmd
	if ghc.Actor == "" {
		ghc.Actor = "nektos/act"
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
	if rc.EventJSON != "" {
		err = json.Unmarshal([]byte(rc.EventJSON), &ghc.Event)
		if err != nil {
			log.Errorf("Unable to Unmarshal event '%s': %v", rc.EventJSON, err)
		}
	}

	// set the branch in the event data
	if rc.Config.DefaultBranch != "" {
		ghc.Event = withDefaultBranch(rc.Config.DefaultBranch, ghc.Event)
	} else {
		ghc.Event = withDefaultBranch("master", ghc.Event)
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

func withDefaultBranch(b string, event map[string]interface{}) map[string]interface{} {
	repoI, ok := event["repository"]
	if !ok {
		repoI = make(map[string]interface{})
	}

	repo, ok := repoI.(map[string]interface{})
	if !ok {
		log.Warnf("unable to set default branch to %v", b)
		return event
	}

	// if the branch is already there return with no changes
	if _, ok = repo["default_branch"]; ok {
		return event
	}

	repo["default_branch"] = b
	event["repository"] = repo

	return event
}

func (rc *RunContext) withGithubEnv(env map[string]string) map[string]string {
	github := rc.getGithubContext()
	env["CI"] = "true"
	env["GITHUB_ENV"] = fmt.Sprintf("%s/%s", filepath.Dir(rc.Config.Workdir), "workflow/envs.txt")
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
	env["GITHUB_SERVER_URL"] = "https://github.com"
	env["GITHUB_API_URL"] = "https://api.github.com"
	env["GITHUB_GRAPHQL_URL"] = "https://api.github.com/graphql"

	job := rc.Run.Job()
	if job.RunsOn() != nil {
		for _, runnerLabel := range job.RunsOn() {
			platformName := rc.ExprEval.Interpolate(runnerLabel)
			if platformName != "" {
				if platformName == "ubuntu-latest" {
					// hardcode current ubuntu-latest since we have no way to check that 'on the fly'
					env["ImageOS"] = "ubuntu20"
				} else {
					platformName = strings.SplitN(strings.Replace(platformName, `-`, ``, 1), `.`, 1)[0]
					env["ImageOS"] = platformName
				}
			}
		}
	}

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
