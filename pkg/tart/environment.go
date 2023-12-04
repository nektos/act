package tart

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
)

type Environment struct {
	container.HostEnvironment
}

func Start() (container.ExecutionsEnvironment, error) {
	env := &Environment{}
	return env, nil
}

func (h *Environment) ToContainerPath(path string) string {
	path = h.HostEnvironment.ToContainerPath(path)
	actPath := filepath.Clean(h.ActPath)
	altPath := filepath.Clean(path)
	if strings.HasPrefix(altPath, actPath) {
		return "/Volumes/My Shared Files/act/" + altPath[len(actPath):]
	}
	return altPath
}

func (e *Environment) Exec(command []string /*cmdline string, */, env map[string]string, user, workdir string) common.Executor {
	return e.ExecWithCmdLine(command, "", env, user, workdir)
}

func (e *Environment) ExecWithCmdLine(command []string, cmdline string, env map[string]string, user, workdir string) common.Executor {
	return func(ctx context.Context) error {
		if err := e.exec(ctx, command, cmdline, env, user, workdir); err != nil {
			select {
			case <-ctx.Done():
				return fmt.Errorf("this step has been cancelled: %w", err)
			default:
				return err
			}
		}
		return nil
	}
}

func (e *Environment) start(ctx context.Context) error {
	gitLabEnv, err := InitEnv()
	if err != nil {
		return err
	}

	config, err := NewConfigFromEnvironment()
	if err != nil {
		return err
	}

	if config.AlwaysPull {
		log.Printf("Pulling the latest version of %s...\n", gitLabEnv.JobImage)
		_, _, err := TartExecWithEnv(ctx, nil, /*additionalPullEnv(gitLabEnv.Registry)*/
			"pull", gitLabEnv.JobImage)
		if err != nil {
			return err
		}
	}

	log.Println("Cloning and configuring a new VM...")
	vm, err := CreateNewVM(ctx, *gitLabEnv, 0, 0)
	if err != nil {
		return err
	}
	var customDirectoryMounts []string
	err = vm.Start(config, gitLabEnv, customDirectoryMounts)
	if err != nil {
		return err
	}

	// Monitor "tart run" command's output so it's not silenced
	go vm.MonitorTartRunOutput()

	log.Println("Waiting for the VM to boot and be SSH-able...")
	ssh, err := vm.OpenSSH(ctx, config)
	if err != nil {
		return err
	}
	defer ssh.Close()

	log.Println("Was able to SSH!")
	return nil
}
func (e *Environment) stop(ctx context.Context) error {
	gitLabEnv, err := InitEnv()
	if err != nil {
		return err
	}

	vm := ExistingVM(*gitLabEnv)

	if err = vm.Stop(); err != nil {
		log.Printf("Failed to stop VM: %v", err)
	}

	if err := vm.Delete(); err != nil {
		log.Printf("Failed to delete VM: %v", err)

		return err
	}

	tartConfig, err := NewConfigFromEnvironment()
	if err != nil {
		return err
	}

	if tartConfig.HostDir {
		if err := os.RemoveAll(gitLabEnv.HostDirPath()); err != nil {
			log.Printf("Failed to clean up %q (temporary directory from the host): %v",
				gitLabEnv.HostDirPath(), err)

			return err
		}
	}

	return nil
}

func (e *Environment) exec(ctx context.Context, command []string, cmdline string, env map[string]string, _, workdir string) error {
	// var wd string
	// if workdir != "" {
	// 	if filepath.IsAbs(workdir) {
	// 		wd = workdir
	// 	} else {
	// 		wd = filepath.Join(e.Path, workdir)
	// 	}
	// } else {
	// 	wd = e.Path
	// }
	gitLabEnv, err := InitEnv()
	if err != nil {
		return err
	}

	vm := ExistingVM(*gitLabEnv)

	// Monitor "tart run" command's output so it's not silenced
	go vm.MonitorTartRunOutput()

	config, err := NewConfigFromEnvironment()
	if err != nil {
		return err
	}

	ssh, err := vm.OpenSSH(ctx, config)
	if err != nil {
		return err
	}
	defer ssh.Close()

	session, err := ssh.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// GitLab script ends with an `exit` command which will terminate the SSH session
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if config.Shell != "" {
		err = session.Start(config.Shell)
	} else {
		err = session.Shell()
	}
	if err != nil {
		return err
	}

	return session.Wait()
}
