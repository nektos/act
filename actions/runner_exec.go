package actions

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"

	"github.com/nektos/act/common"
	"github.com/nektos/act/container"
	log "github.com/sirupsen/logrus"
)

func (runner *runnerImpl) newActionExecutor(actionName string) common.Executor {
	action, err := runner.workflows.getAction(actionName)
	if err != nil {
		return common.NewErrorExecutor(err)
	}

	env := make(map[string]string)
	for _, applier := range []environmentApplier{action, runner} {
		applier.applyEnvironment(env)
	}
	env["GITHUB_ACTION"] = actionName

	logger := newActionLogger(actionName, runner.config.Dryrun)
	log.Debugf("Using '%s' for action '%s'", action.Uses, actionName)

	in := container.DockerExecutorInput{
		Ctx:    runner.config.Ctx,
		Logger: logger,
		Dryrun: runner.config.Dryrun,
	}

	var image string
	executors := make([]common.Executor, 0)
	if imageRef, ok := parseImageReference(action.Uses); ok {
		executors = append(executors, container.NewDockerPullExecutor(container.NewDockerPullExecutorInput{
			DockerExecutorInput: in,
			Image:               imageRef,
		}))
		image = imageRef
	} else if contextDir, imageTag, ok := parseImageLocal(runner.config.WorkingDir, action.Uses); ok {
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
			Dryrun: runner.config.Dryrun,
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

	ghReader, err := runner.createGithubTarball()
	if err != nil {
		return common.NewErrorExecutor(err)
	}
	randSuffix := randString(6)
	containerName := regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(actionName, "-")
	if len(containerName)+len(randSuffix)+1 > 30 {
		containerName = containerName[:(30 - (len(randSuffix) + 1))]
	}

	envList := make([]string, 0)
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	executors = append(executors, container.NewDockerRunExecutor(container.NewDockerRunExecutorInput{
		DockerExecutorInput: in,
		Cmd:                 action.Args,
		Entrypoint:          action.Runs,
		Image:               image,
		WorkingDir:          "/github/workspace",
		Env:                 envList,
		Name:                fmt.Sprintf("%s-%s", containerName, randSuffix),
		Binds: []string{
			fmt.Sprintf("%s:%s", runner.config.WorkingDir, "/github/workspace"),
			fmt.Sprintf("%s:%s", runner.tempDir, "/github/home"),
			fmt.Sprintf("%s:%s", "/var/run/docker.sock", "/var/run/docker.sock"),
		},
		Content: map[string]io.Reader{"/github": ghReader},
	}))

	return common.NewPipelineExecutor(executors...)
}

func (runner *runnerImpl) applyEnvironment(env map[string]string) {
	repoPath := runner.config.WorkingDir

	_, workflowName, _ := runner.workflows.getWorkflow(runner.config.EventName)

	env["HOME"] = "/github/home"
	env["GITHUB_ACTOR"] = "nektos/act"
	env["GITHUB_EVENT_PATH"] = "/github/workflow/event.json"
	env["GITHUB_WORKSPACE"] = "/github/workspace"
	env["GITHUB_WORKFLOW"] = workflowName
	env["GITHUB_EVENT_NAME"] = runner.config.EventName

	_, rev, err := common.FindGitRevision(repoPath)
	if err != nil {
		log.Warningf("unable to get git revision: %v", err)
	} else {
		env["GITHUB_SHA"] = rev
	}

	repo, err := common.FindGithubRepo(repoPath)
	if err != nil {
		log.Warningf("unable to get git repo: %v", err)
	} else {
		env["GITHUB_REPOSITORY"] = repo
	}

	branch, err := common.FindGitBranch(repoPath)
	if err != nil {
		log.Warningf("unable to get git branch: %v", err)
	} else {
		env["GITHUB_REF"] = fmt.Sprintf("refs/heads/%s", branch)
	}

}

func (runner *runnerImpl) createGithubTarball() (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var files = []struct {
		Name string
		Mode int64
		Body string
	}{
		{"workflow/event.json", 0644, runner.eventJSON},
	}
	for _, file := range files {
		log.Debugf("Writing entry to tarball %s len:%d", file.Name, len(runner.eventJSON))
		hdr := &tar.Header{
			Name: file.Name,
			Mode: file.Mode,
			Size: int64(len(runner.eventJSON)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(runner.eventJSON)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil

}

const letterBytes = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randString(slen int) string {
	b := make([]byte, slen)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}
