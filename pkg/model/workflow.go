package model

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// Workflow is the structure of the files in .github/workflows
type Workflow struct {
	Name string            `yaml:"name"`
	On   string            `yaml:"on"`
	Env  map[string]string `yaml:"env"`
	Jobs map[string]*Job   `yaml:"jobs"`
}

// Job is the structure of one job in a workflow
type Job struct {
	Name           string                    `yaml:"name"`
	Needs          []string                  `yaml:"needs"`
	RunsOn         string                    `yaml:"runs-on"`
	Env            map[string]string         `yaml:"env"`
	If             string                    `yaml:"if"`
	Steps          []*Step                   `yaml:"steps"`
	TimeoutMinutes int64                     `yaml:"timeout-minutes"`
	Container      *ContainerSpec            `yaml:"container"`
	Services       map[string]*ContainerSpec `yaml:"services"`
}

// ContainerSpec is the specification of the container to use for the job
type ContainerSpec struct {
	Image      string            `yaml:"image"`
	Env        map[string]string `yaml:"env"`
	Ports      []int             `yaml:"ports"`
	Volumes    []string          `yaml:"volumes"`
	Options    string            `yaml:"options"`
	Entrypoint string
	Args       string
}

// Step is the structure of one step in a job
type Step struct {
	ID               string            `yaml:"id"`
	If               string            `yaml:"if"`
	Name             string            `yaml:"name"`
	Uses             string            `yaml:"uses"`
	Run              string            `yaml:"run"`
	WorkingDirectory string            `yaml:"working-directory"`
	Shell            string            `yaml:"shell"`
	Env              map[string]string `yaml:"env"`
	With             map[string]string `yaml:"with"`
	ContinueOnError  bool              `yaml:"continue-on-error"`
	TimeoutMinutes   int64             `yaml:"timeout-minutes"`
}

// GetEnv gets the env for a step
func (s *Step) GetEnv() map[string]string {
	rtnEnv := make(map[string]string)
	for k, v := range s.Env {
		rtnEnv[k] = v
	}
	for k, v := range s.With {
		envKey := fmt.Sprintf("INPUT_%s", strings.ToUpper(k))
		envKey = regexp.MustCompile("[^A-Z0-9]").ReplaceAllString(envKey, "_")
		rtnEnv[envKey] = v
	}
	return rtnEnv
}

// StepType describes what type of step we are about to run
type StepType int

const (
	// StepTypeRun is all steps that have a `run` attribute
	StepTypeRun StepType = iota

	//StepTypeUsesDockerURL is all steps that have a `uses` that is of the form `docker://...`
	StepTypeUsesDockerURL

	//StepTypeUsesActionLocal is all steps that have a `uses` that is a reference to a github repo
	StepTypeUsesActionLocal

	//StepTypeUsesActionRemote is all steps that have a `uses` that is a local action in a subdirectory
	StepTypeUsesActionRemote
)

// Type returns the type of the step
func (s *Step) Type() StepType {
	if s.Run != "" {
		return StepTypeRun
	} else if strings.HasPrefix(s.Uses, "docker://") {
		return StepTypeUsesDockerURL
	} else if strings.HasPrefix(s.Uses, "./") {
		return StepTypeUsesActionLocal
	}
	return StepTypeUsesActionRemote
}

// ReadWorkflow returns a list of jobs for a given workflow file reader
func ReadWorkflow(in io.Reader) (*Workflow, error) {
	w := new(Workflow)
	err := yaml.NewDecoder(in).Decode(w)
	return w, err
}

// GetJob will get a job by name in the workflow
func (w *Workflow) GetJob(jobID string) *Job {
	for id, j := range w.Jobs {
		if jobID == id {
			return j
		}
	}
	return nil
}

// GetJobIDs will get all the job names in the workflow
func (w *Workflow) GetJobIDs() []string {
	ids := make([]string, 0)
	for id := range w.Jobs {
		ids = append(ids, id)
	}
	return ids
}
