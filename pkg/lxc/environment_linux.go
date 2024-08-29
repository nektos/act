package lxc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"

	_ "embed"
)

type Environment struct {
	container.HostEnvironment
	Config   Config
	Env      *Env
	Miscpath string
}

// "/Volumes/My Shared Files/act/"
func (e *Environment) ToHostPath(path string) string {
	actPath := filepath.Clean("/private/tmp/act/")
	altPath := filepath.Clean(path)
	if strings.HasPrefix(altPath, actPath) {
		return e.Miscpath + altPath[len(actPath):]
	}
	return altPath
}

func (e *Environment) ToContainerPath(path string) string {
	path = e.HostEnvironment.ToContainerPath(path)
	actPath := filepath.Clean(e.Miscpath)
	altPath := filepath.Clean(path)
	if strings.HasPrefix(altPath, actPath) {
		return "/private/tmp/act/" + altPath[len(actPath):]
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

func (e *Environment) Start(b bool) common.Executor {
	return e.HostEnvironment.Start(b).Then(func(ctx context.Context) error {
		return e.start(ctx)
	})
}

//go:embed start_template.sh
var rawStartTemplate string

var startTemplate = template.Must(template.New("stop").Parse(rawStartTemplate))

//go:embed stop_template.sh
var rawStopTemplate string

var stopTemplate = template.Must(template.New("stop").Parse(rawStopTemplate))

func (e *Environment) start(ctx context.Context) error {
	actEnv := e.Env

	config := e.Config

	if config.AlwaysPull {
		log.Printf("Pulling the latest version of %s...\n", actEnv.JobImage)
		// ...
	}

	log.Println("Cloning and configuring a new VM...")

	var startScript bytes.Buffer
	if err := startTemplate.Execute(&startScript, struct {
		Name     string
		Template string
		Release  string
		Repo     string
		Root     string
		TmpDir   string
		Script   string
	}{
		Name:     e.Env.JobID,
		Template: "debian",
		Release:  "bullseye",
		Repo:     "", // step.Environment["CI_REPO"],
		Root:     e.Miscpath,
		TmpDir:   e.TmpDir,
		Script:   "", // "commands-" + step.Name,
	}); err != nil {
		return err
	}
	// e.Copy(rc.JobContainer.GetActPath()+"/", &container.FileEntry{
	// 	Name: "workflow/start-lxc.sh",
	// 	Mode: 0755,
	// 	Body: startScript.String(),
	// }),
	// rc.JobContainer.Exec([]string{rc.JobContainer.GetActPath() + "/workflow/start-lxc.sh"}, map[string]string{}, "root", rc.Config.Workdir),
	return nil
}
func (e *Environment) Stop(ctx context.Context) error {
	log.Println("Stop VM?")

	// ...

	return nil
}

func (e *Environment) Remove() common.Executor {
	return func(ctx context.Context) error {
		_ = e.Stop(ctx)
		log.Println("Remove VM?")
		if e.CleanUp != nil {
			e.CleanUp()
		}
		_ = os.RemoveAll(e.Path)
		return e.Close()(ctx)
	}
}
func (e *Environment) exec(ctx context.Context, command []string, _ string, env map[string]string, _, workdir string) error {
	fcommand := []string{"/usr/bin/lxc-attach"}
	for k, v := range env {
		fcommand = append(fcommand, "--set-var", k+"="+v)
	}
	fcommand = append(fcommand, "--name", e.Env.JobID, "--")
	fcommand = append(fcommand, command...)
	return nil
}

func (e *Environment) GetActPath() string {
	return e.ToContainerPath(e.HostEnvironment.GetActPath())
}

func (e *Environment) Copy(destPath string, files ...*container.FileEntry) common.Executor {
	return e.HostEnvironment.Copy(e.ToHostPath(destPath), files...)
}
func (e *Environment) CopyTarStream(ctx context.Context, destPath string, tarStream io.Reader) error {
	return e.HostEnvironment.CopyTarStream(ctx, e.ToHostPath(destPath), tarStream)
}
func (e *Environment) CopyDir(destPath string, srcPath string, useGitIgnore bool) common.Executor {
	return e.HostEnvironment.CopyDir(e.ToHostPath(destPath), srcPath, useGitIgnore)
}
func (e *Environment) GetContainerArchive(ctx context.Context, srcPath string) (io.ReadCloser, error) {
	return e.HostEnvironment.GetContainerArchive(ctx, e.ToHostPath(srcPath))
}

func (e *Environment) GetRunnerContext(ctx context.Context) map[string]interface{} {
	rctx := e.HostEnvironment.GetRunnerContext(ctx)
	rctx["temp"] = e.ToContainerPath(e.TmpDir)
	rctx["tool_cache"] = e.ToContainerPath(e.ToolCache)
	return rctx
}
