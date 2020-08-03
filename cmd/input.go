package cmd

import (
	"log"
	"path/filepath"
)

// Input contains the input for the root command
type Input struct {
	actor           string
	workdir         string
	workflowsPath   string
	eventPath       string
	reuseContainers bool
	bindWorkdir     bool
	secrets         []string
	platforms       []string
	dryrun          bool
	forcePull       bool
	noOutput        bool
	envfile         string
	secretfile      string
	privileged      bool
}

func (i *Input) resolve(path string) string {
	basedir, err := filepath.Abs(i.workdir)
	if err != nil {
		log.Fatal(err)
	}
	if path == "" {
		return path
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(basedir, path)
	}
	return path
}

// Envfile returns path to .env
func (i *Input) Envfile() string {
	return i.resolve(i.envfile)
}

// Secretfile returns path to secrets
func (i *Input) Secretfile() string {
	return i.resolve(i.secretfile)
}

// Workdir returns path to workdir
func (i *Input) Workdir() string {
	return i.resolve(".")
}

// WorkflowsPath returns path to workflow file(s)
func (i *Input) WorkflowsPath() string {
	return i.resolve(i.workflowsPath)
}

// EventPath returns the path to events file
func (i *Input) EventPath() string {
	return i.resolve(i.eventPath)
}
