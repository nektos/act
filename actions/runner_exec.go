package actions

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/actions/workflow-parser/model"
	"github.com/nektos/act/common"
	"github.com/nektos/act/container"
	log "github.com/sirupsen/logrus"
)

func (runner *runnerImpl) newActionExecutor(actionName string) common.Executor {
	action := runner.workflowConfig.GetAction(actionName)
	if action == nil {
		return common.NewErrorExecutor(fmt.Errorf("Unable to find action named '%s'", actionName))
	}

	executors := make([]common.Executor, 0)
	image, err := runner.addImageExecutor(action, &executors)
	if err != nil {
		return common.NewErrorExecutor(err)
	}

	err = runner.addRunExecutor(action, image, &executors)
	if err != nil {
		return common.NewErrorExecutor(err)
	}

	return common.NewPipelineExecutor(executors...)
}

func (runner *runnerImpl) addImageExecutor(action *model.Action, executors *[]common.Executor) (string, error) {
	var image string
	logger := newActionLogger(action.Identifier, runner.config.Dryrun)
	log.Debugf("Using '%s' for action '%s'", action.Uses, action.Identifier)

	in := container.DockerExecutorInput{
		Ctx:    runner.config.Ctx,
		Logger: logger,
		Dryrun: runner.config.Dryrun,
	}
	switch uses := action.Uses.(type) {

	case *model.UsesDockerImage:
		image = uses.Image

		pull := runner.config.ForcePull
		if !pull {
			imageExists, err := container.ImageExistsLocally(runner.config.Ctx, image)
			log.Debugf("Image exists? %v", imageExists)
			if err != nil {
				return "", fmt.Errorf("unable to determine if image already exists for image %q", image)
			}

			if !imageExists {
				pull = true
			}
		}

		if pull {
			*executors = append(*executors, container.NewDockerPullExecutor(container.NewDockerPullExecutorInput{
				DockerExecutorInput: in,
				Image:               image,
			}))
		}

	case *model.UsesPath:
		contextDir := filepath.Join(runner.config.WorkingDir, uses.String())
		sha, _, err := common.FindGitRevision(contextDir)
		if err != nil {
			log.Warnf("Unable to determine git revision: %v", err)
			sha = "latest"
		}
		image = fmt.Sprintf("%s:%s", filepath.Base(contextDir), sha)

		*executors = append(*executors, container.NewDockerBuildExecutor(container.NewDockerBuildExecutorInput{
			DockerExecutorInput: in,
			ContextDir:          contextDir,
			ImageTag:            image,
		}))

	case *model.UsesRepository:
		image = fmt.Sprintf("%s:%s", filepath.Base(uses.Repository), uses.Ref)
		cloneURL := fmt.Sprintf("https://github.com/%s", uses.Repository)

		cloneDir := filepath.Join(os.TempDir(), "act", action.Uses.String())
		*executors = append(*executors, common.NewGitCloneExecutor(common.NewGitCloneExecutorInput{
			URL:    cloneURL,
			Ref:    uses.Ref,
			Dir:    cloneDir,
			Logger: logger,
			Dryrun: runner.config.Dryrun,
		}))

		contextDir := filepath.Join(cloneDir, uses.Path)
		*executors = append(*executors, container.NewDockerBuildExecutor(container.NewDockerBuildExecutorInput{
			DockerExecutorInput: in,
			ContextDir:          contextDir,
			ImageTag:            image,
		}))

	default:
		return "", fmt.Errorf("unable to determine executor type for image '%s'", action.Uses)
	}

	return image, nil
}

func (runner *runnerImpl) addRunExecutor(action *model.Action, image string, executors *[]common.Executor) error {
	logger := newActionLogger(action.Identifier, runner.config.Dryrun)
	log.Debugf("Using '%s' for action '%s'", action.Uses, action.Identifier)

	in := container.DockerExecutorInput{
		Ctx:    runner.config.Ctx,
		Logger: logger,
		Dryrun: runner.config.Dryrun,
	}

	env := make(map[string]string)
	for _, applier := range []environmentApplier{newActionEnvironmentApplier(action), runner} {
		applier.applyEnvironment(env)
	}
	env["GITHUB_ACTION"] = action.Identifier

	ghReader, err := runner.createGithubTarball()
	if err != nil {
		return err
	}

	envList := make([]string, 0)
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	var cmd, entrypoint []string
	if action.Args != nil {
		cmd = action.Args.Split()
	}
	if action.Runs != nil {
		entrypoint = action.Runs.Split()
	}
	*executors = append(*executors, container.NewDockerRunExecutor(container.NewDockerRunExecutorInput{
		DockerExecutorInput: in,
		Cmd:                 cmd,
		Entrypoint:          entrypoint,
		Image:               image,
		WorkingDir:          "/github/workspace",
		Env:                 envList,
		Name:                runner.createContainerName(action.Identifier),
		Binds: []string{
			fmt.Sprintf("%s:%s", runner.config.WorkingDir, "/github/workspace"),
			fmt.Sprintf("%s:%s", runner.tempDir, "/github/home"),
			fmt.Sprintf("%s:%s", "/var/run/docker.sock", "/var/run/docker.sock"),
		},
		Content:         map[string]io.Reader{"/github": ghReader},
		ReuseContainers: runner.config.ReuseContainers,
	}))

	return nil
}

func (runner *runnerImpl) applyEnvironment(env map[string]string) {
	repoPath := runner.config.WorkingDir

	workflows := runner.workflowConfig.GetWorkflows(runner.config.EventName)
	if len(workflows) == 0 {
		return
	}
	workflowName := workflows[0].Identifier

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

	ref, err := common.FindGitRef(repoPath)
	if err != nil {
		log.Warningf("unable to get git ref: %v", err)
	} else {
		log.Infof("using github ref: %s", ref)
		env["GITHUB_REF"] = ref
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

func (runner *runnerImpl) createContainerName(actionName string) string {
	containerName := regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(actionName, "-")

	prefix := fmt.Sprintf("%s-", trimToLen(filepath.Base(runner.config.WorkingDir), 10))
	suffix := ""
	containerName = trimToLen(containerName, 30-(len(prefix)+len(suffix)))
	return fmt.Sprintf("%s%s%s", prefix, containerName, suffix)
}

func trimToLen(s string, l int) string {
	if len(s) > l {
		return s[:l]
	}
	return s
}
