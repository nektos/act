package cmd

import (
	"log"
	"path/filepath"
)

// Input contains the input for the root command
type Input struct {
	workingDir      string
	workflowsPath   string
	eventPath       string
	reuseContainers bool
	dryrun          bool
	forcePull       bool
}

func (i *Input) resolve(path string) string {
	basedir, err := filepath.Abs(i.workingDir)
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

// WorkflowsPath returns path to workflows
func (i *Input) WorkflowsPath() string {
	return i.resolve(i.workflowsPath)
}

// EventPath returns the path to events file
func (i *Input) EventPath() string {
	return i.resolve(i.eventPath)
}
