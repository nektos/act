package model

import (
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/common"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Workflow is the structure of the files in .github/workflows
type Workflow struct {
	Name     string            `yaml:"name"`
	RawOn    yaml.Node         `yaml:"on"`
	Env      map[string]string `yaml:"env"`
	Jobs     map[string]*Job   `yaml:"jobs"`
	Defaults Defaults          `yaml:"defaults"`
}

// On events for the workflow
func (w *Workflow) On() []string {
	switch w.RawOn.Kind {
	case yaml.ScalarNode:
		var val string
		err := w.RawOn.Decode(&val)
		if err != nil {
			log.Fatal(err)
		}
		return []string{val}
	case yaml.SequenceNode:
		var val []string
		err := w.RawOn.Decode(&val)
		if err != nil {
			log.Fatal(err)
		}
		return val
	case yaml.MappingNode:
		var val map[string]interface{}
		err := w.RawOn.Decode(&val)
		if err != nil {
			log.Fatal(err)
		}
		var keys []string
		for k := range val {
			keys = append(keys, k)
		}
		return keys
	}
	return nil
}

// Job is the structure of one job in a workflow
type Job struct {
	Name           string                    `yaml:"name"`
	RawNeeds       yaml.Node                 `yaml:"needs"`
	RawRunsOn      yaml.Node                 `yaml:"runs-on"`
	Env            map[string]string         `yaml:"env"`
	If             string                    `yaml:"if"`
	Steps          []*Step                   `yaml:"steps"`
	TimeoutMinutes int64                     `yaml:"timeout-minutes"`
	Services       map[string]*ContainerSpec `yaml:"services"`
	Strategy       *Strategy                 `yaml:"strategy"`
	RawContainer   yaml.Node                 `yaml:"container"`
	Defaults       Defaults                  `yaml:"defaults"`
}

// Strategy for the job
type Strategy struct {
	FailFast    bool                     `yaml:"fail-fast"`
	MaxParallel int                      `yaml:"max-parallel"`
	Matrix      map[string][]interface{} `yaml:"matrix"`
}

// Default settings that will apply to all steps in the job or workflow
type Defaults struct {
	Run RunDefaults `yaml:"run"`
}

// Defaults for all run steps in the job or workflow
type RunDefaults struct {
	Shell            string `yaml:"shell"`
	WorkingDirectory string `yaml:"working-directory"`
}

// Container details for the job
func (j *Job) Container() *ContainerSpec {
	var val *ContainerSpec
	switch j.RawContainer.Kind {
	case yaml.ScalarNode:
		val = new(ContainerSpec)
		err := j.RawContainer.Decode(&val.Image)
		if err != nil {
			log.Fatal(err)
		}
	case yaml.MappingNode:
		val = new(ContainerSpec)
		err := j.RawContainer.Decode(val)
		if err != nil {
			log.Fatal(err)
		}
	}
	return val
}

// Needs list for Job
func (j *Job) Needs() []string {
	switch j.RawNeeds.Kind {
	case yaml.ScalarNode:
		var val string
		err := j.RawNeeds.Decode(&val)
		if err != nil {
			log.Fatal(err)
		}
		return []string{val}
	case yaml.SequenceNode:
		var val []string
		err := j.RawNeeds.Decode(&val)
		if err != nil {
			log.Fatal(err)
		}
		return val
	}
	return nil
}

// RunsOn list for Job
func (j *Job) RunsOn() []string {
	switch j.RawRunsOn.Kind {
	case yaml.ScalarNode:
		var val string
		err := j.RawRunsOn.Decode(&val)
		if err != nil {
			log.Fatal(err)
		}
		return []string{val}
	case yaml.SequenceNode:
		var val []string
		err := j.RawRunsOn.Decode(&val)
		if err != nil {
			log.Fatal(err)
		}
		return val
	}
	return nil
}

// GetMatrixes returns the matrix cross product
func (j *Job) GetMatrixes() []map[string]interface{} {
	matrixes := make([]map[string]interface{}, 0)
	if j.Strategy != nil {
		includes := make([]map[string]interface{}, 0)
		for _, v := range j.Strategy.Matrix["include"] {
			includes = append(includes, v.(map[string]interface{}))
		}
		delete(j.Strategy.Matrix, "include")

		excludes := make([]map[string]interface{}, 0)
		for _, v := range j.Strategy.Matrix["exclude"] {
			excludes = append(excludes, v.(map[string]interface{}))
		}
		delete(j.Strategy.Matrix, "exclude")

		matrixProduct := common.CartesianProduct(j.Strategy.Matrix)

	MATRIX:
		for _, matrix := range matrixProduct {
			for _, exclude := range excludes {
				if commonKeysMatch(matrix, exclude) {
					log.Debugf("Skipping matrix '%v' due to exclude '%v'", matrix, exclude)
					continue MATRIX
				}
			}
			matrixes = append(matrixes, matrix)
		}
		for _, include := range includes {
			log.Debugf("Adding include '%v'", include)
			matrixes = append(matrixes, include)
		}
	} else {
		matrixes = append(matrixes, make(map[string]interface{}))
	}
	return matrixes
}

func commonKeysMatch(a map[string]interface{}, b map[string]interface{}) bool {
	for aKey, aVal := range a {
		if bVal, ok := b[aKey]; ok && !reflect.DeepEqual(aVal, bVal) {
			return false
		}
	}
	return true
}

// ContainerSpec is the specification of the container to use for the job
type ContainerSpec struct {
	Image      string            `yaml:"image"`
	Env        map[string]string `yaml:"env"`
	Ports      []string          `yaml:"ports"`
	Volumes    []string          `yaml:"volumes"`
	Options    string            `yaml:"options"`
	Entrypoint string
	Args       string
	Name       string
	Reuse      bool
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

// String gets the name of step
func (s *Step) String() string {
	if s.Name != "" {
		return s.Name
	} else if s.Uses != "" {
		return s.Uses
	} else if s.Run != "" {
		return s.Run
	}
	return s.ID
}

// GetEnv gets the env for a step
func (s *Step) GetEnv() map[string]string {
	rtnEnv := make(map[string]string)
	for k, v := range s.Env {
		rtnEnv[k] = v
	}
	for k, v := range s.With {
		envKey := regexp.MustCompile("[^A-Z0-9-]").ReplaceAllString(strings.ToUpper(k), "_")
		envKey = fmt.Sprintf("INPUT_%s", strings.ToUpper(envKey))
		rtnEnv[envKey] = v
	}
	return rtnEnv
}

// ShellCommand returns the command for the shell
func (s *Step) ShellCommand() string {
	shellCommand := ""

	//Reference: https://github.com/actions/runner/blob/8109c962f09d9acc473d92c595ff43afceddb347/src/Runner.Worker/Handlers/ScriptHandlerHelpers.cs#L9-L17
	switch s.Shell {
	case "", "bash":
		shellCommand = "bash --login --noprofile --norc -e -o pipefail {0}"
	case "pwsh":
		shellCommand = "pwsh -login -command . '{0}'"
	case "python":
		shellCommand = "python {0}"
	case "sh":
		shellCommand = "sh -l -e -c {0}"
	case "cmd":
		shellCommand = "%ComSpec% /D /E:ON /V:OFF /S /C \"CALL \"{0}\"\""
	case "powershell":
		shellCommand = "powershell -command . '{0}'"
	default:
		shellCommand = s.Shell
	}
	return shellCommand
}

// StepType describes what type of step we are about to run
type StepType int

const (
	// StepTypeRun is all steps that have a `run` attribute
	StepTypeRun StepType = iota

	// StepTypeUsesDockerURL is all steps that have a `uses` that is of the form `docker://...`
	StepTypeUsesDockerURL

	// StepTypeUsesActionLocal is all steps that have a `uses` that is a local action in a subdirectory
	StepTypeUsesActionLocal

	// StepTypeUsesActionRemote is all steps that have a `uses` that is a reference to a github repo
	StepTypeUsesActionRemote

	// StepTypeInvalid is for steps that have invalid step action
	StepTypeInvalid
)

// Type returns the type of the step
func (s *Step) Type() StepType {
	if s.Run != "" {
		if s.Uses != "" {
			return StepTypeInvalid
		}
		return StepTypeRun
	} else if strings.HasPrefix(s.Uses, "docker://") {
		return StepTypeUsesDockerURL
	} else if strings.HasPrefix(s.Uses, "./") {
		return StepTypeUsesActionLocal
	}
	return StepTypeUsesActionRemote
}

func (s *Step) Validate() error {
	if s.Type() != StepTypeRun {
		return fmt.Errorf("(StepID: %s): Unexpected value 'uses'", s.String())
	} else if s.Shell == "" {
		return fmt.Errorf("(StepID: %s): Required property is missing: 'shell'", s.String())
	}
	return nil
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
			if j.Name == "" {
				j.Name = id
			}
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
