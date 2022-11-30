package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/opencontainers/selinux/go-selinux"
	log "github.com/sirupsen/logrus"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/common/git"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/exprparser"
	"github.com/nektos/act/pkg/model"
)

// RunContext contains info about current job
type RunContext struct {
	Name                string
	Config              *Config
	Matrix              map[string]interface{}
	Run                 *model.Run
	EventJSON           string
	Env                 map[string]string
	ExtraPath           []string
	CurrentStep         string
	StepResults         map[string]*model.StepResult
	ExprEval            ExpressionEvaluator
	JobContainer        container.ExecutionsEnvironment
	OutputMappings      map[MappableOutput]MappableOutput
	JobName             string
	ActionPath          string
	Parent              *RunContext
	Masks               []string
	cleanUpJobContainer common.Executor
}

func (rc *RunContext) AddMask(mask string) {
	rc.Masks = append(rc.Masks, mask)
}

type MappableOutput struct {
	StepID     string
	OutputName string
}

func (rc *RunContext) String() string {
	return fmt.Sprintf("%s/%s", rc.Run.Workflow.Name, rc.Name)
}

// GetEnv returns the env for the context
func (rc *RunContext) GetEnv() map[string]string {
	if rc.Env == nil {
		rc.Env = map[string]string{}
		if rc.Run != nil && rc.Run.Workflow != nil && rc.Config != nil {
			job := rc.Run.Job()
			if job != nil {
				rc.Env = mergeMaps(rc.Run.Workflow.Env, job.Environment(), rc.Config.Env)
			}
		}
	}
	rc.Env["ACT"] = "true"
	return rc.Env
}

func (rc *RunContext) jobContainerName() string {
	return createContainerName("act", rc.String())
}

// Returns the binds and mounts for the container, resolving paths as appopriate
func (rc *RunContext) GetBindsAndMounts() ([]string, map[string]string) {
	name := rc.jobContainerName()

	if rc.Config.ContainerDaemonSocket == "" {
		rc.Config.ContainerDaemonSocket = "/var/run/docker.sock"
	}

	binds := []string{
		fmt.Sprintf("%s:%s", rc.Config.ContainerDaemonSocket, "/var/run/docker.sock"),
	}

	ext := container.LinuxContainerEnvironmentExtensions{}

	mounts := map[string]string{
		"act-toolcache": "/toolcache",
		name + "-env":   ext.GetActPath(),
	}

	if job := rc.Run.Job(); job != nil {
		if container := job.Container(); container != nil {
			for _, v := range container.Volumes {
				if !strings.Contains(v, ":") || filepath.IsAbs(v) {
					// Bind anonymous volume or host file.
					binds = append(binds, v)
				} else {
					// Mount existing volume.
					paths := strings.SplitN(v, ":", 2)
					mounts[paths[0]] = paths[1]
				}
			}
		}
	}

	if rc.Config.BindWorkdir {
		bindModifiers := ""
		if runtime.GOOS == "darwin" {
			bindModifiers = ":delegated"
		}
		if selinux.GetEnabled() {
			bindModifiers = ":z"
		}
		binds = append(binds, fmt.Sprintf("%s:%s%s", rc.Config.Workdir, ext.ToContainerPath(rc.Config.Workdir), bindModifiers))
	} else {
		mounts[name] = ext.ToContainerPath(rc.Config.Workdir)
	}

	return binds, mounts
}

func (rc *RunContext) startHostEnvironment() common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		rawLogger := logger.WithField("raw_output", true)
		logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) bool {
			if rc.Config.LogOutput {
				rawLogger.Infof("%s", s)
			} else {
				rawLogger.Debugf("%s", s)
			}
			return true
		})
		cacheDir := rc.ActionCacheDir()
		randBytes := make([]byte, 8)
		_, _ = rand.Read(randBytes)
		miscpath := filepath.Join(cacheDir, hex.EncodeToString(randBytes))
		actPath := filepath.Join(miscpath, "act")
		if err := os.MkdirAll(actPath, 0777); err != nil {
			return err
		}
		path := filepath.Join(miscpath, "hostexecutor")
		if err := os.MkdirAll(path, 0777); err != nil {
			return err
		}
		runnerTmp := filepath.Join(miscpath, "tmp")
		if err := os.MkdirAll(runnerTmp, 0777); err != nil {
			return err
		}
		toolCache := filepath.Join(cacheDir, "tool_cache")
		rc.JobContainer = &container.HostEnvironment{
			Path:      path,
			TmpDir:    runnerTmp,
			ToolCache: toolCache,
			Workdir:   rc.Config.Workdir,
			ActPath:   actPath,
			CleanUp: func() {
				os.RemoveAll(miscpath)
			},
			StdOut: logWriter,
		}
		rc.cleanUpJobContainer = rc.JobContainer.Remove()
		rc.Env["RUNNER_TOOL_CACHE"] = toolCache
		rc.Env["RUNNER_OS"] = runtime.GOOS
		rc.Env["RUNNER_ARCH"] = runtime.GOARCH
		rc.Env["RUNNER_TEMP"] = runnerTmp
		for _, env := range os.Environ() {
			i := strings.Index(env, "=")
			if i > 0 {
				rc.Env[env[0:i]] = env[i+1:]
			}
		}

		return common.NewPipelineExecutor(
			rc.JobContainer.Copy(rc.JobContainer.GetActPath()+"/", &container.FileEntry{
				Name: "workflow/event.json",
				Mode: 0644,
				Body: rc.EventJSON,
			}, &container.FileEntry{
				Name: "workflow/envs.txt",
				Mode: 0666,
				Body: "",
			}, &container.FileEntry{
				Name: "workflow/paths.txt",
				Mode: 0666,
				Body: "",
			}),
		)(ctx)
	}
}

func (rc *RunContext) startJobContainer() common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		image := rc.platformImage(ctx)
		rawLogger := logger.WithField("raw_output", true)
		logWriter := common.NewLineWriter(rc.commandHandler(ctx), func(s string) bool {
			if rc.Config.LogOutput {
				rawLogger.Infof("%s", s)
			} else {
				rawLogger.Debugf("%s", s)
			}
			return true
		})

		username, password, err := rc.handleCredentials(ctx)
		if err != nil {
			return fmt.Errorf("failed to handle credentials: %s", err)
		}

		logger.Infof("\U0001f680  Start image=%s", image)
		name := rc.jobContainerName()

		envList := make([]string, 0)

		envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TOOL_CACHE", "/opt/hostedtoolcache"))
		envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_OS", "Linux"))
		envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_ARCH", container.RunnerArch(ctx)))
		envList = append(envList, fmt.Sprintf("%s=%s", "RUNNER_TEMP", "/tmp"))
		envList = append(envList, fmt.Sprintf("%s=%s", "LANG", "C.UTF-8")) // Use same locale as GitHub Actions

		ext := container.LinuxContainerEnvironmentExtensions{}
		binds, mounts := rc.GetBindsAndMounts()

		rc.cleanUpJobContainer = func(ctx context.Context) error {
			if rc.JobContainer != nil && !rc.Config.ReuseContainers {
				return rc.JobContainer.Remove().
					Then(container.NewDockerVolumeRemoveExecutor(rc.jobContainerName(), false)).
					Then(container.NewDockerVolumeRemoveExecutor(rc.jobContainerName()+"-env", false))(ctx)
			}
			return nil
		}

		rc.JobContainer = container.NewContainer(&container.NewContainerInput{
			Cmd:         nil,
			Entrypoint:  []string{"/usr/bin/tail", "-f", "/dev/null"},
			WorkingDir:  ext.ToContainerPath(rc.Config.Workdir),
			Image:       image,
			Username:    username,
			Password:    password,
			Name:        name,
			Env:         envList,
			Mounts:      mounts,
			NetworkMode: "host",
			Binds:       binds,
			Stdout:      logWriter,
			Stderr:      logWriter,
			Privileged:  rc.Config.Privileged,
			UsernsMode:  rc.Config.UsernsMode,
			Platform:    rc.Config.ContainerArchitecture,
			Options:     rc.options(ctx),
		})
		if rc.JobContainer == nil {
			return errors.New("Failed to create job container")
		}

		return common.NewPipelineExecutor(
			rc.JobContainer.Pull(rc.Config.ForcePull),
			rc.stopJobContainer(),
			rc.JobContainer.Create(rc.Config.ContainerCapAdd, rc.Config.ContainerCapDrop),
			rc.JobContainer.Start(false),
			rc.JobContainer.UpdateFromImageEnv(&rc.Env),
			rc.JobContainer.UpdateFromEnv("/etc/environment", &rc.Env),
			rc.JobContainer.Copy(rc.JobContainer.GetActPath()+"/", &container.FileEntry{
				Name: "workflow/event.json",
				Mode: 0644,
				Body: rc.EventJSON,
			}, &container.FileEntry{
				Name: "workflow/envs.txt",
				Mode: 0666,
				Body: "",
			}, &container.FileEntry{
				Name: "workflow/paths.txt",
				Mode: 0666,
				Body: "",
			}),
		)(ctx)
	}
}

func (rc *RunContext) execJobContainer(cmd []string, env map[string]string, user, workdir string) common.Executor {
	return func(ctx context.Context) error {
		return rc.JobContainer.Exec(cmd, env, user, workdir)(ctx)
	}
}

// stopJobContainer removes the job container (if it exists) and its volume (if it exists) if !rc.Config.ReuseContainers
func (rc *RunContext) stopJobContainer() common.Executor {
	return func(ctx context.Context) error {
		if rc.cleanUpJobContainer != nil && !rc.Config.ReuseContainers {
			return rc.cleanUpJobContainer(ctx)
		}
		return nil
	}
}

// Prepare the mounts and binds for the worker

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

// Interpolate outputs after a job is done
func (rc *RunContext) interpolateOutputs() common.Executor {
	return func(ctx context.Context) error {
		ee := rc.NewExpressionEvaluator(ctx)
		for k, v := range rc.Run.Job().Outputs {
			interpolated := ee.Interpolate(ctx, v)
			if v != interpolated {
				rc.Run.Job().Outputs[k] = interpolated
			}
		}
		return nil
	}
}

func (rc *RunContext) startContainer() common.Executor {
	return func(ctx context.Context) error {
		image := rc.platformImage(ctx)
		if strings.EqualFold(image, "-self-hosted") {
			return rc.startHostEnvironment()(ctx)
		}
		return rc.startJobContainer()(ctx)
	}
}

func (rc *RunContext) stopContainer() common.Executor {
	return rc.stopJobContainer()
}

func (rc *RunContext) closeContainer() common.Executor {
	return func(ctx context.Context) error {
		if rc.JobContainer != nil {
			return rc.JobContainer.Close()(ctx)
		}
		return nil
	}
}

func (rc *RunContext) matrix() map[string]interface{} {
	return rc.Matrix
}

func (rc *RunContext) result(result string) {
	rc.Run.Job().Result = result
}

func (rc *RunContext) steps() []*model.Step {
	return rc.Run.Job().Steps
}

// Executor returns a pipeline executor for all the steps in the job
func (rc *RunContext) Executor() common.Executor {
	return func(ctx context.Context) error {
		isEnabled, err := rc.isEnabled(ctx)
		if err != nil {
			return err
		}

		if isEnabled {
			return newJobExecutor(rc, &stepFactoryImpl{}, rc)(ctx)
		}

		return nil
	}
}

func (rc *RunContext) platformImage(ctx context.Context) string {
	job := rc.Run.Job()

	c := job.Container()
	if c != nil {
		return rc.ExprEval.Interpolate(ctx, c.Image)
	}

	if job.RunsOn() == nil {
		common.Logger(ctx).Errorf("'runs-on' key not defined in %s", rc.String())
	}

	for _, runnerLabel := range job.RunsOn() {
		platformName := rc.ExprEval.Interpolate(ctx, runnerLabel)
		image := rc.Config.Platforms[strings.ToLower(platformName)]
		if image != "" {
			return image
		}
	}

	return ""
}

func (rc *RunContext) options(ctx context.Context) string {
	job := rc.Run.Job()
	c := job.Container()
	if c == nil {
		return ""
	}

	return c.Options
}

func (rc *RunContext) isEnabled(ctx context.Context) (bool, error) {
	job := rc.Run.Job()
	l := common.Logger(ctx)
	runJob, err := EvalBool(ctx, rc.ExprEval, job.If.Value, exprparser.DefaultStatusCheckSuccess)
	if err != nil {
		return false, fmt.Errorf("  \u274C  Error in if-expression: \"if: %s\" (%s)", job.If.Value, err)
	}
	if !runJob {
		l.WithField("jobResult", "skipped").Debugf("Skipping job '%s' due to '%s'", job.Name, job.If.Value)
		return false, nil
	}

	img := rc.platformImage(ctx)
	if img == "" {
		if job.RunsOn() == nil {
			l.Errorf("'runs-on' key not defined in %s", rc.String())
		}

		for _, runnerLabel := range job.RunsOn() {
			platformName := rc.ExprEval.Interpolate(ctx, runnerLabel)
			l.Infof("\U0001F6A7  Skipping unsupported platform -- Try running with `-P %+v=...`", platformName)
		}
		return false, nil
	}
	return true, nil
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

func (rc *RunContext) getJobContext() *model.JobContext {
	jobStatus := "success"
	for _, stepStatus := range rc.StepResults {
		if stepStatus.Conclusion == model.StepStatusFailure {
			jobStatus = "failure"
			break
		}
	}
	return &model.JobContext{
		Status: jobStatus,
	}
}

func (rc *RunContext) getStepsContext() map[string]*model.StepResult {
	return rc.StepResults
}

func (rc *RunContext) getGithubContext(ctx context.Context) *model.GithubContext {
	logger := common.Logger(ctx)
	ghc := &model.GithubContext{
		Event:            make(map[string]interface{}),
		Workflow:         rc.Run.Workflow.Name,
		RunID:            rc.Config.Env["GITHUB_RUN_ID"],
		RunNumber:        rc.Config.Env["GITHUB_RUN_NUMBER"],
		Actor:            rc.Config.Actor,
		EventName:        rc.Config.EventName,
		Action:           rc.CurrentStep,
		Token:            rc.Config.Token,
		ActionPath:       rc.ActionPath,
		RepositoryOwner:  rc.Config.Env["GITHUB_REPOSITORY_OWNER"],
		RetentionDays:    rc.Config.Env["GITHUB_RETENTION_DAYS"],
		RunnerPerflog:    rc.Config.Env["RUNNER_PERFLOG"],
		RunnerTrackingID: rc.Config.Env["RUNNER_TRACKING_ID"],
	}
	if rc.JobContainer != nil {
		ghc.EventPath = rc.JobContainer.GetActPath() + "/workflow/event.json"
		ghc.Workspace = rc.JobContainer.ToContainerPath(rc.Config.Workdir)
	}

	if ghc.RunID == "" {
		ghc.RunID = "1"
	}

	if ghc.RunNumber == "" {
		ghc.RunNumber = "1"
	}

	if ghc.RetentionDays == "" {
		ghc.RetentionDays = "0"
	}

	if ghc.RunnerPerflog == "" {
		ghc.RunnerPerflog = "/dev/null"
	}

	// Backwards compatibility for configs that require
	// a default rather than being run as a cmd
	if ghc.Actor == "" {
		ghc.Actor = "nektos/act"
	}

	repoPath := rc.Config.Workdir
	repo, err := git.FindGithubRepo(ctx, repoPath, rc.Config.GitHubInstance, rc.Config.RemoteName)
	if err != nil {
		logger.Warningf("unable to get git repo: %v", err)
	} else {
		ghc.Repository = repo
		if ghc.RepositoryOwner == "" {
			ghc.RepositoryOwner = strings.Split(repo, "/")[0]
		}
	}

	if rc.EventJSON != "" {
		err = json.Unmarshal([]byte(rc.EventJSON), &ghc.Event)
		if err != nil {
			logger.Errorf("Unable to Unmarshal event '%s': %v", rc.EventJSON, err)
		}
	}

	if ghc.EventName == "pull_request" || ghc.EventName == "pull_request_target" {
		ghc.BaseRef = asString(nestedMapLookup(ghc.Event, "pull_request", "base", "ref"))
		ghc.HeadRef = asString(nestedMapLookup(ghc.Event, "pull_request", "head", "ref"))
	}

	ghc.SetRefAndSha(ctx, rc.Config.DefaultBranch, repoPath)

	// https://docs.github.com/en/actions/learn-github-actions/environment-variables
	if strings.HasPrefix(ghc.Ref, "refs/tags/") {
		ghc.RefType = "tag"
		ghc.RefName = ghc.Ref[len("refs/tags/"):]
	} else if strings.HasPrefix(ghc.Ref, "refs/heads/") {
		ghc.RefType = "branch"
		ghc.RefName = ghc.Ref[len("refs/heads/"):]
	}

	return ghc
}

func isLocalCheckout(ghc *model.GithubContext, step *model.Step) bool {
	if step.Type() == model.StepTypeInvalid {
		// This will be errored out by the executor later, we need this here to avoid a null panic though
		return false
	}
	if step.Type() != model.StepTypeUsesActionRemote {
		return false
	}
	remoteAction := newRemoteAction(step.Uses)
	if remoteAction == nil {
		// IsCheckout() will nil panic if we dont bail out early
		return false
	}
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

func (rc *RunContext) withGithubEnv(ctx context.Context, github *model.GithubContext, env map[string]string) map[string]string {
	env["CI"] = "true"
	env["GITHUB_ENV"] = rc.JobContainer.GetActPath() + "/workflow/envs.txt"
	env["GITHUB_PATH"] = rc.JobContainer.GetActPath() + "/workflow/paths.txt"
	env["GITHUB_WORKFLOW"] = github.Workflow
	env["GITHUB_RUN_ID"] = github.RunID
	env["GITHUB_RUN_NUMBER"] = github.RunNumber
	env["GITHUB_ACTION"] = github.Action
	env["GITHUB_ACTION_PATH"] = github.ActionPath
	env["GITHUB_ACTION_REPOSITORY"] = github.ActionRepository
	env["GITHUB_ACTION_REF"] = github.ActionRef
	env["GITHUB_ACTIONS"] = "true"
	env["GITHUB_ACTOR"] = github.Actor
	env["GITHUB_REPOSITORY"] = github.Repository
	env["GITHUB_EVENT_NAME"] = github.EventName
	env["GITHUB_EVENT_PATH"] = github.EventPath
	env["GITHUB_WORKSPACE"] = github.Workspace
	env["GITHUB_SHA"] = github.Sha
	env["GITHUB_REF"] = github.Ref
	env["GITHUB_REF_NAME"] = github.RefName
	env["GITHUB_REF_TYPE"] = github.RefType
	env["GITHUB_TOKEN"] = github.Token
	env["GITHUB_SERVER_URL"] = "https://github.com"
	env["GITHUB_API_URL"] = "https://api.github.com"
	env["GITHUB_GRAPHQL_URL"] = "https://api.github.com/graphql"
	env["GITHUB_BASE_REF"] = github.BaseRef
	env["GITHUB_HEAD_REF"] = github.HeadRef
	env["GITHUB_JOB"] = rc.JobName
	env["GITHUB_REPOSITORY_OWNER"] = github.RepositoryOwner
	env["GITHUB_RETENTION_DAYS"] = github.RetentionDays
	env["RUNNER_PERFLOG"] = github.RunnerPerflog
	env["RUNNER_TRACKING_ID"] = github.RunnerTrackingID
	if rc.Config.GitHubInstance != "github.com" {
		env["GITHUB_SERVER_URL"] = fmt.Sprintf("https://%s", rc.Config.GitHubInstance)
		env["GITHUB_API_URL"] = fmt.Sprintf("https://%s/api/v3", rc.Config.GitHubInstance)
		env["GITHUB_GRAPHQL_URL"] = fmt.Sprintf("https://%s/api/graphql", rc.Config.GitHubInstance)
	}

	if rc.Config.ArtifactServerPath != "" {
		setActionRuntimeVars(rc, env)
	}

	job := rc.Run.Job()
	if job.RunsOn() != nil {
		for _, runnerLabel := range job.RunsOn() {
			platformName := rc.ExprEval.Interpolate(ctx, runnerLabel)
			if platformName != "" {
				if platformName == "ubuntu-latest" {
					// hardcode current ubuntu-latest since we have no way to check that 'on the fly'
					env["ImageOS"] = "ubuntu20"
				} else {
					platformName = strings.SplitN(strings.Replace(platformName, `-`, ``, 1), `.`, 2)[0]
					env["ImageOS"] = platformName
				}
			}
		}
	}

	return env
}

func setActionRuntimeVars(rc *RunContext, env map[string]string) {
	actionsRuntimeURL := os.Getenv("ACTIONS_RUNTIME_URL")
	if actionsRuntimeURL == "" {
		actionsRuntimeURL = fmt.Sprintf("http://%s:%s/", common.GetOutboundIP().String(), rc.Config.ArtifactServerPort)
	}
	env["ACTIONS_RUNTIME_URL"] = actionsRuntimeURL

	actionsRuntimeToken := os.Getenv("ACTIONS_RUNTIME_TOKEN")
	if actionsRuntimeToken == "" {
		actionsRuntimeToken = "token"
	}
	env["ACTIONS_RUNTIME_TOKEN"] = actionsRuntimeToken
}

func (rc *RunContext) handleCredentials(ctx context.Context) (username, password string, err error) {
	// TODO: remove below 2 lines when we can release act with breaking changes
	username = rc.Config.Secrets["DOCKER_USERNAME"]
	password = rc.Config.Secrets["DOCKER_PASSWORD"]

	container := rc.Run.Job().Container()
	if container == nil || container.Credentials == nil {
		return
	}

	if container.Credentials != nil && len(container.Credentials) != 2 {
		err = fmt.Errorf("invalid property count for key 'credentials:'")
		return
	}

	ee := rc.NewExpressionEvaluator(ctx)
	if username = ee.Interpolate(ctx, container.Credentials["username"]); username == "" {
		err = fmt.Errorf("failed to interpolate container.credentials.username")
		return
	}
	if password = ee.Interpolate(ctx, container.Credentials["password"]); password == "" {
		err = fmt.Errorf("failed to interpolate container.credentials.password")
		return
	}

	if container.Credentials["username"] == "" || container.Credentials["password"] == "" {
		err = fmt.Errorf("container.credentials cannot be empty")
		return
	}

	return username, password, err
}
