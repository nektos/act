package container

import (
	"context"
	"fmt"
	"github.com/nektos/act/pkg/common"
	"os"
	"os/exec"
	"strings"
)

type NewHostContainerInput struct {
	WorkingDir string
}

type hostContainer struct {
	input *NewHostContainerInput
}

func NewHost(hostInput *NewHostContainerInput) Container {
	hc := new(hostContainer)
	hc.input = hostInput

	return hc
}

func (hc *hostContainer) Create() common.Executor {
	return common.
		NewDebugExecutor("%susing host working dir=%s", logPrefix, hc.input.WorkingDir).
		Then(
			common.NewParallelExecutor(
				hc.executeCommand(fmt.Sprintf("mkdir -p %s", hc.input.WorkingDir), map[string]string{}),
				hc.executeCommand("pwd", map[string]string{}),
				hc.executeCommand("mkdir -p /actions/", map[string]string{}),
			))
}

func (hc *hostContainer) executeCommand(command string, env map[string]string) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)

		words := strings.Fields(command)

		cmd := exec.Command(words[0], words[1:]...)
		cmd.Env = os.Environ()

		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		logger.Infof("  \u2601  running command: %s with env %s", command, env, cmd.Dir)

		return cmd.Run()
	}
}

func (hc *hostContainer) Copy(destPath string, files ...*FileEntry) common.Executor {
	panic("implement me")
}

func (hc *hostContainer) CopyDir(destPath string, srcPath string) common.Executor {
	return hc.executeCommand(fmt.Sprintf("cp -R %s %s", srcPath, destPath), map[string]string{})
}

func (hc *hostContainer) Pull(forcePull bool) common.Executor {
	panic("implement me")
}

func (hc *hostContainer) Start(attach bool) common.Executor {
	panic("implement me")
}

func (hc *hostContainer) Exec(command []string, env map[string]string) common.Executor {
	envList := make([]string, 0)
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	return hc.executeCommand(strings.Join(command, " "), env)
}

func (hc *hostContainer) Remove() common.Executor {
	panic("implement me")
}
