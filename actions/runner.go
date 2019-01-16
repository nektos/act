package actions

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/howeyc/gopass"
	"github.com/nektos/act/common"
	"github.com/nektos/act/container"
	log "github.com/sirupsen/logrus"
)

var secretCache map[string]string

func (wFile *workflowsFile) ListEvents() []string {
	log.Debugf("Listing all events")
	events := make([]string, 0)
	for _, w := range wFile.Workflow {
		events = append(events, w.On)
	}

	// sort the list based on depth of dependencies
	sort.Slice(events, func(i, j int) bool {
		return events[i] < events[j]
	})

	return events
}

func (wFile *workflowsFile) GraphEvent(eventName string) ([][]string, error) {
	log.Debugf("Listing actions for event '%s'", eventName)
	workflow, _, err := wFile.getWorkflow(eventName)
	if err != nil {
		return nil, err
	}
	return wFile.newExecutionGraph(workflow.Resolves...), nil
}

func (wFile *workflowsFile) RunAction(ctx context.Context, dryrun bool, actionName string) error {
	log.Debugf("Running action '%s'", actionName)
	return wFile.newActionExecutor(ctx, dryrun, "", actionName)()
}

func (wFile *workflowsFile) RunEvent(ctx context.Context, dryrun bool, eventName string) error {
	log.Debugf("Running event '%s'", eventName)
	workflow, _, err := wFile.getWorkflow(eventName)
	if err != nil {
		return err
	}

	log.Debugf("Running actions %s -> %s", eventName, workflow.Resolves)
	return wFile.newActionExecutor(ctx, dryrun, eventName, workflow.Resolves...)()
}

func (wFile *workflowsFile) getWorkflow(eventName string) (*workflowDef, string, error) {
	var rtn workflowDef
	for wName, w := range wFile.Workflow {
		if w.On == eventName {
			rtn = w
			return &rtn, wName, nil
		}
	}
	return nil, "", fmt.Errorf("unsupported event: %v", eventName)
}

func (wFile *workflowsFile) getAction(actionName string) (*actionDef, error) {
	if a, ok := wFile.Action[actionName]; ok {
		return &a, nil
	}
	return nil, fmt.Errorf("unsupported action: %v", actionName)
}

func (wFile *workflowsFile) Close() {
	os.RemoveAll(wFile.TempDir)
}

// return a pipeline that is run in series.  pipeline is a list of steps to run in parallel
func (wFile *workflowsFile) newExecutionGraph(actionNames ...string) [][]string {
	// first, build a list of all the necessary actions to run, and their dependencies
	actionDependencies := make(map[string][]string)
	for len(actionNames) > 0 {
		newActionNames := make([]string, 0)
		for _, aName := range actionNames {
			// make sure we haven't visited this action yet
			if _, ok := actionDependencies[aName]; !ok {
				actionDependencies[aName] = wFile.Action[aName].Needs
				newActionNames = append(newActionNames, wFile.Action[aName].Needs...)
			}
		}
		actionNames = newActionNames
	}

	// next, build an execution graph
	graph := make([][]string, 0)
	for len(actionDependencies) > 0 {
		stage := make([]string, 0)
		for aName, aDeps := range actionDependencies {
			// make sure all deps are in the graph already
			if listInLists(aDeps, graph...) {
				stage = append(stage, aName)
				delete(actionDependencies, aName)
			}
		}
		if len(stage) == 0 {
			log.Fatalf("Unable to build dependency graph!")
		}
		graph = append(graph, stage)
	}

	return graph
}

// return true iff all strings in srcList exist in at least one of the searchLists
func listInLists(srcList []string, searchLists ...[]string) bool {
	for _, src := range srcList {
		found := false
		for _, searchList := range searchLists {
			for _, search := range searchList {
				if src == search {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (wFile *workflowsFile) newActionExecutor(ctx context.Context, dryrun bool, eventName string, actionNames ...string) common.Executor {
	graph := wFile.newExecutionGraph(actionNames...)

	pipeline := make([]common.Executor, 0)
	for _, actions := range graph {
		stage := make([]common.Executor, 0)
		for _, actionName := range actions {
			action, err := wFile.getAction(actionName)
			if err != nil {
				return common.NewErrorExecutor(err)
			}
			actionExecutor := action.asExecutor(ctx, dryrun, wFile.WorkingDir, wFile.TempDir, actionName, wFile.setupEnvironment(eventName, actionName, dryrun))
			stage = append(stage, actionExecutor)
		}
		pipeline = append(pipeline, common.NewParallelExecutor(stage...))
	}

	return common.NewPipelineExecutor(pipeline...)
}

func (action *actionDef) asExecutor(ctx context.Context, dryrun bool, workingDir string, tempDir string, actionName string, env []string) common.Executor {
	logger := newActionLogger(actionName, dryrun)
	log.Debugf("Using '%s' for action '%s'", action.Uses, actionName)

	in := container.DockerExecutorInput{
		Ctx:    ctx,
		Logger: logger,
		Dryrun: dryrun,
	}

	var image string
	executors := make([]common.Executor, 0)
	if imageRef, ok := parseImageReference(action.Uses); ok {
		executors = append(executors, container.NewDockerPullExecutor(container.NewDockerPullExecutorInput{
			DockerExecutorInput: in,
			Image:               imageRef,
		}))
		image = imageRef
	} else if contextDir, imageTag, ok := parseImageLocal(workingDir, action.Uses); ok {
		executors = append(executors, container.NewDockerBuildExecutor(container.NewDockerBuildExecutorInput{
			DockerExecutorInput: in,
			ContextDir:          contextDir,
			ImageTag:            imageTag,
		}))
		image = imageTag
	} else if cloneURL, ref, path, ok := parseImageGithub(action.Uses); ok {
		cloneDir := filepath.Join(os.TempDir(), "act", action.Uses)
		executors = append(executors, common.NewGitCloneExecutor(common.NewGitCloneExecutorInput{
			URL:    cloneURL,
			Ref:    ref,
			Dir:    cloneDir,
			Logger: logger,
			Dryrun: dryrun,
		}))

		contextDir := filepath.Join(cloneDir, path)
		imageTag := fmt.Sprintf("%s:%s", filepath.Base(cloneURL.Path), ref)

		executors = append(executors, container.NewDockerBuildExecutor(container.NewDockerBuildExecutorInput{
			DockerExecutorInput: in,
			ContextDir:          contextDir,
			ImageTag:            imageTag,
		}))
		image = imageTag
	} else {
		return common.NewErrorExecutor(fmt.Errorf("unable to determine executor type for image '%s'", action.Uses))
	}

	ghReader, err := action.createGithubTarball()
	if err != nil {
		return common.NewErrorExecutor(err)
	}
	randSuffix := randString(6)
	containerName := regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(actionName, "-")
	if len(containerName)+len(randSuffix)+1 > 30 {
		containerName = containerName[:(30 - (len(randSuffix) + 1))]
	}
	executors = append(executors, container.NewDockerRunExecutor(container.NewDockerRunExecutorInput{
		DockerExecutorInput: in,
		Cmd:                 action.Args,
		Entrypoint:          action.Runs,
		Image:               image,
		WorkingDir:          "/github/workspace",
		Env:                 env,
		Name:                fmt.Sprintf("%s-%s", containerName, randSuffix),
		Binds: []string{
			fmt.Sprintf("%s:%s", workingDir, "/github/workspace"),
			fmt.Sprintf("%s:%s", tempDir, "/github/home"),
			fmt.Sprintf("%s:%s", "/var/run/docker.sock", "/var/run/docker.sock"),
		},
		Content: map[string]io.Reader{"/github": ghReader},
	}))

	return common.NewPipelineExecutor(executors...)
}

const letterBytes = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randString(slen int) string {
	b := make([]byte, slen)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

func (action *actionDef) createGithubTarball() (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var files = []struct {
		Name string
		Mode int64
		Body string
	}{
		{"workflow/event.json", 0644, "{}"},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: file.Mode,
			Size: int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(file.Body)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil

}

func (wFile *workflowsFile) setupEnvironment(eventName string, actionName string, dryrun bool) []string {
	env := make([]string, 0)
	repoPath := wFile.WorkingDir

	_, workflowName, _ := wFile.getWorkflow(eventName)

	env = append(env, fmt.Sprintf("HOME=/github/home"))
	env = append(env, fmt.Sprintf("GITHUB_ACTOR=nektos/act"))
	env = append(env, fmt.Sprintf("GITHUB_EVENT_PATH=/github/workflow/event.json"))
	env = append(env, fmt.Sprintf("GITHUB_WORKSPACE=/github/workspace"))
	env = append(env, fmt.Sprintf("GITHUB_WORKFLOW=%s", workflowName))
	env = append(env, fmt.Sprintf("GITHUB_EVENT_NAME=%s", eventName))
	env = append(env, fmt.Sprintf("GITHUB_ACTION=%s", actionName))

	_, rev, err := common.FindGitRevision(repoPath)
	if err != nil {
		log.Warningf("unable to get git revision: %v", err)
	} else {
		env = append(env, fmt.Sprintf("GITHUB_SHA=%s", rev))
	}

	repo, err := common.FindGithubRepo(repoPath)
	if err != nil {
		log.Warningf("unable to get git repo: %v", err)
	} else {
		env = append(env, fmt.Sprintf("GITHUB_REPOSITORY=%s", repo))
	}

	branch, err := common.FindGitBranch(repoPath)
	if err != nil {
		log.Warningf("unable to get git branch: %v", err)
	} else {
		env = append(env, fmt.Sprintf("GITHUB_REF=refs/heads/%s", branch))
	}

	action, err := wFile.getAction(actionName)
	if err == nil && !dryrun {
		action.applyEnvironmentSecrets(&env)
	}

	return env
}

func (action *actionDef) applyEnvironmentSecrets(env *[]string) {
	if action != nil {
		for envKey, envValue := range action.Env {
			*env = append(*env, fmt.Sprintf("%s=%s", envKey, envValue))
		}

		for _, secret := range action.Secrets {
			if secretVal, ok := os.LookupEnv(secret); ok {
				*env = append(*env, fmt.Sprintf("%s=%s", secret, secretVal))
			} else {
				if secretCache == nil {
					secretCache = make(map[string]string)
				}

				if secretCache[secret] == "" {
					fmt.Printf("Provide value for '%s': ", secret)
					val, err := gopass.GetPasswdMasked()
					if err != nil {
						log.Fatal("abort")
					}

					secretCache[secret] = string(val)
				}
				*env = append(*env, fmt.Sprintf("%s=%s", secret, secretCache[secret]))
			}
		}
	}
}

// imageURL is the directory where a `Dockerfile` should exist
func parseImageLocal(workingDir string, contextDir string) (contextDirOut string, tag string, ok bool) {
	if !filepath.IsAbs(contextDir) {
		contextDir = filepath.Join(workingDir, contextDir)
	}
	if _, err := os.Stat(filepath.Join(contextDir, "Dockerfile")); os.IsNotExist(err) {
		log.Debugf("Ignoring missing Dockerfile '%s/Dockerfile'", contextDir)
		return "", "", false
	}

	sha, _, err := common.FindGitRevision(contextDir)
	if err != nil {
		log.Warnf("Unable to determine git revision: %v", err)
		sha = "latest"
	}
	return contextDir, fmt.Sprintf("%s:%s", filepath.Base(contextDir), sha), true
}

// imageURL is the URL for a docker repo
func parseImageReference(image string) (ref string, ok bool) {
	imageURL, err := url.Parse(image)
	if err != nil {
		log.Debugf("Unable to parse image as url: %v", err)
		return "", false
	}
	if imageURL.Scheme != "docker" {
		log.Debugf("Ignoring non-docker ref '%s'", imageURL.String())
		return "", false
	}

	return fmt.Sprintf("%s%s", imageURL.Host, imageURL.Path), true
}

// imageURL is the directory where a `Dockerfile` should exist
func parseImageGithub(image string) (cloneURL *url.URL, ref string, path string, ok bool) {
	re := regexp.MustCompile("^([^/@]+)/([^/@]+)(/([^@]*))?(@(.*))?$")
	matches := re.FindStringSubmatch(image)

	if matches == nil {
		return nil, "", "", false
	}

	cloneURL, err := url.Parse(fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2]))
	if err != nil {
		log.Debugf("Unable to parse as URL: %v", err)
		return nil, "", "", false
	}

	resp, err := http.Head(cloneURL.String())
	if resp.StatusCode >= 400 || err != nil {
		log.Debugf("Unable to HEAD URL %s status=%v err=%v", cloneURL.String(), resp.StatusCode, err)
		return nil, "", "", false
	}

	ref = matches[6]
	if ref == "" {
		ref = "master"
	}

	path = matches[4]
	if path == "" {
		path = "."
	}

	return cloneURL, ref, path, true
}
