package model

import (
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/nektos/act/pkg/common"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Workflow is the structure of the files in .github/workflows
type Workflow struct {
	File     string
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

func (w *Workflow) OnEvent(event string) interface{} {
	if w.RawOn.Kind == yaml.MappingNode {
		var val map[string]interface{}
		if !decodeNode(w.RawOn, &val) {
			return nil
		}
		return val[event]
	}
	return nil
}

type WorkflowDispatchInput struct {
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	Type        string   `yaml:"type"`
	Options     []string `yaml:"options"`
}

type WorkflowDispatch struct {
	Inputs map[string]WorkflowDispatchInput `yaml:"inputs"`
}

func (w *Workflow) WorkflowDispatchConfig() *WorkflowDispatch {
	switch w.RawOn.Kind {
	case yaml.ScalarNode:
		var val string
		if !decodeNode(w.RawOn, &val) {
			return nil
		}
		if val == "workflow_dispatch" {
			return &WorkflowDispatch{}
		}
	case yaml.SequenceNode:
		var val []string
		if !decodeNode(w.RawOn, &val) {
			return nil
		}
		for _, v := range val {
			if v == "workflow_dispatch" {
				return &WorkflowDispatch{}
			}
		}
	case yaml.MappingNode:
		var val map[string]yaml.Node
		if !decodeNode(w.RawOn, &val) {
			return nil
		}

		n, found := val["workflow_dispatch"]
		var workflowDispatch WorkflowDispatch
		if found && decodeNode(n, &workflowDispatch) {
			return &workflowDispatch
		}
	default:
		return nil
	}
	return nil
}

type WorkflowCallInput struct {
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default"`
	Type        string `yaml:"type"`
}

type WorkflowCallOutput struct {
	Description string `yaml:"description"`
	Value       string `yaml:"value"`
}

type WorkflowCall struct {
	Inputs  map[string]WorkflowCallInput  `yaml:"inputs"`
	Outputs map[string]WorkflowCallOutput `yaml:"outputs"`
}

type WorkflowCallResult struct {
	Outputs map[string]string
}

func (w *Workflow) WorkflowCallConfig() *WorkflowCall {
	if w.RawOn.Kind != yaml.MappingNode {
		// The callers expect for "on: workflow_call" and "on: [ workflow_call ]" a non nil return value
		return &WorkflowCall{}
	}

	var val map[string]yaml.Node
	if !decodeNode(w.RawOn, &val) {
		return &WorkflowCall{}
	}

	var config WorkflowCall
	node := val["workflow_call"]
	if !decodeNode(node, &config) {
		return &WorkflowCall{}
	}

	return &config
}

// Job is the structure of one job in a workflow
type Job struct {
	Name           string                    `yaml:"name"`
	RawNeeds       yaml.Node                 `yaml:"needs"`
	RawRunsOn      yaml.Node                 `yaml:"runs-on"`
	Env            yaml.Node                 `yaml:"env"`
	If             yaml.Node                 `yaml:"if"`
	Steps          []*Step                   `yaml:"steps"`
	TimeoutMinutes string                    `yaml:"timeout-minutes"`
	Services       map[string]*ContainerSpec `yaml:"services"`
	Strategy       *Strategy                 `yaml:"strategy"`
	RawContainer   yaml.Node                 `yaml:"container"`
	Defaults       Defaults                  `yaml:"defaults"`
	Outputs        map[string]string         `yaml:"outputs"`
	Uses           string                    `yaml:"uses"`
	With           map[string]interface{}    `yaml:"with"`
	RawSecrets     yaml.Node                 `yaml:"secrets"`
	Result         string
}

// Strategy for the job
type Strategy struct {
	FailFast          bool
	MaxParallel       int
	FailFastString    string    `yaml:"fail-fast"`
	MaxParallelString string    `yaml:"max-parallel"`
	RawMatrix         yaml.Node `yaml:"matrix"`
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

// GetMaxParallel sets default and returns value for `max-parallel`
func (s Strategy) GetMaxParallel() int {
	// MaxParallel default value is `GitHub will maximize the number of jobs run in parallel depending on the available runners on GitHub-hosted virtual machines`
	// So I take the liberty to hardcode default limit to 4 and this is because:
	// 1: tl;dr: self-hosted does only 1 parallel job - https://github.com/actions/runner/issues/639#issuecomment-825212735
	// 2: GH has 20 parallel job limit (for free tier) - https://github.com/github/docs/blob/3ae84420bd10997bb5f35f629ebb7160fe776eae/content/actions/reference/usage-limits-billing-and-administration.md?plain=1#L45
	// 3: I want to add support for MaxParallel to act and 20! parallel jobs is a bit overkill IMHO
	maxParallel := 4
	if s.MaxParallelString != "" {
		var err error
		if maxParallel, err = strconv.Atoi(s.MaxParallelString); err != nil {
			log.Errorf("Failed to parse 'max-parallel' option: %v", err)
		}
	}
	return maxParallel
}

// GetFailFast sets default and returns value for `fail-fast`
func (s Strategy) GetFailFast() bool {
	// FailFast option is true by default: https://github.com/github/docs/blob/3ae84420bd10997bb5f35f629ebb7160fe776eae/content/actions/reference/workflow-syntax-for-github-actions.md?plain=1#L1107
	failFast := true
	log.Debug(s.FailFastString)
	if s.FailFastString != "" {
		var err error
		if failFast, err = strconv.ParseBool(s.FailFastString); err != nil {
			log.Errorf("Failed to parse 'fail-fast' option: %v", err)
		}
	}
	return failFast
}

func (j *Job) InheritSecrets() bool {
	if j.RawSecrets.Kind != yaml.ScalarNode {
		return false
	}

	var val string
	if !decodeNode(j.RawSecrets, &val) {
		return false
	}

	return val == "inherit"
}

func (j *Job) Secrets() map[string]string {
	if j.RawSecrets.Kind != yaml.MappingNode {
		return nil
	}

	var val map[string]string
	if !decodeNode(j.RawSecrets, &val) {
		return nil
	}

	return val
}

// Container details for the job
func (j *Job) Container() *ContainerSpec {
	var val *ContainerSpec
	switch j.RawContainer.Kind {
	case yaml.ScalarNode:
		val = new(ContainerSpec)
		if !decodeNode(j.RawContainer, &val.Image) {
			return nil
		}
	case yaml.MappingNode:
		val = new(ContainerSpec)
		if !decodeNode(j.RawContainer, val) {
			return nil
		}
	}
	return val
}

// Needs list for Job
func (j *Job) Needs() []string {
	switch j.RawNeeds.Kind {
	case yaml.ScalarNode:
		var val string
		if !decodeNode(j.RawNeeds, &val) {
			return nil
		}
		return []string{val}
	case yaml.SequenceNode:
		var val []string
		if !decodeNode(j.RawNeeds, &val) {
			return nil
		}
		return val
	}
	return nil
}

// RunsOn list for Job
func (j *Job) RunsOn() []string {
	switch j.RawRunsOn.Kind {
	case yaml.MappingNode:
		var val struct {
			Group  string
			Labels yaml.Node
		}

		if !decodeNode(j.RawRunsOn, &val) {
			return nil
		}

		labels := nodeAsStringSlice(val.Labels)

		if val.Group != "" {
			labels = append(labels, val.Group)
		}

		return labels
	default:
		return nodeAsStringSlice(j.RawRunsOn)
	}
}

func nodeAsStringSlice(node yaml.Node) []string {
	switch node.Kind {
	case yaml.ScalarNode:
		var val string
		if !decodeNode(node, &val) {
			return nil
		}
		return []string{val}
	case yaml.SequenceNode:
		var val []string
		if !decodeNode(node, &val) {
			return nil
		}
		return val
	}
	return nil
}

func environment(yml yaml.Node) map[string]string {
	env := make(map[string]string)
	if yml.Kind == yaml.MappingNode {
		if !decodeNode(yml, &env) {
			return nil
		}
	}
	return env
}

// Environment returns string-based key=value map for a job
func (j *Job) Environment() map[string]string {
	return environment(j.Env)
}

// Matrix decodes RawMatrix YAML node
func (j *Job) Matrix() map[string][]interface{} {
	if j.Strategy.RawMatrix.Kind == yaml.MappingNode {
		var val map[string][]interface{}
		if !decodeNode(j.Strategy.RawMatrix, &val) {
			return nil
		}
		return val
	}
	return nil
}

// GetMatrixes returns the matrix cross product
// It skips includes and hard fails excludes for non-existing keys
//
//nolint:gocyclo
func (j *Job) GetMatrixes() ([]map[string]interface{}, error) {
	matrixes := make([]map[string]interface{}, 0)
	if j.Strategy != nil {
		j.Strategy.FailFast = j.Strategy.GetFailFast()
		j.Strategy.MaxParallel = j.Strategy.GetMaxParallel()

		if m := j.Matrix(); m != nil {
			includes := make([]map[string]interface{}, 0)
			extraIncludes := make([]map[string]interface{}, 0)
			for _, v := range m["include"] {
				switch t := v.(type) {
				case []interface{}:
					for _, i := range t {
						i := i.(map[string]interface{})
						extraInclude := true
						for k := range i {
							if _, ok := m[k]; ok {
								includes = append(includes, i)
								extraInclude = false
								break
							}
						}
						if extraInclude {
							extraIncludes = append(extraIncludes, i)
						}
					}
				case interface{}:
					v := v.(map[string]interface{})
					extraInclude := true
					for k := range v {
						if _, ok := m[k]; ok {
							includes = append(includes, v)
							extraInclude = false
							break
						}
					}
					if extraInclude {
						extraIncludes = append(extraIncludes, v)
					}
				}
			}
			delete(m, "include")

			excludes := make([]map[string]interface{}, 0)
			for _, e := range m["exclude"] {
				e := e.(map[string]interface{})
				for k := range e {
					if _, ok := m[k]; ok {
						excludes = append(excludes, e)
					} else {
						// We fail completely here because that's what GitHub does for non-existing matrix keys, fail on exclude, silent skip on include
						return nil, fmt.Errorf("the workflow is not valid. Matrix exclude key %q does not match any key within the matrix", k)
					}
				}
			}
			delete(m, "exclude")

			matrixProduct := common.CartesianProduct(m)
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
				matched := false
				for _, matrix := range matrixes {
					if commonKeysMatch2(matrix, include, m) {
						matched = true
						log.Debugf("Adding include values '%v' to existing entry", include)
						for k, v := range include {
							matrix[k] = v
						}
					}
				}
				if !matched {
					extraIncludes = append(extraIncludes, include)
				}
			}
			for _, include := range extraIncludes {
				log.Debugf("Adding include '%v'", include)
				matrixes = append(matrixes, include)
			}
			if len(matrixes) == 0 {
				matrixes = append(matrixes, make(map[string]interface{}))
			}
		} else {
			matrixes = append(matrixes, make(map[string]interface{}))
		}
	} else {
		matrixes = append(matrixes, make(map[string]interface{}))
		log.Debugf("Empty Strategy, matrixes=%v", matrixes)
	}
	return matrixes, nil
}

func commonKeysMatch(a map[string]interface{}, b map[string]interface{}) bool {
	for aKey, aVal := range a {
		if bVal, ok := b[aKey]; ok && !reflect.DeepEqual(aVal, bVal) {
			return false
		}
	}
	return true
}

func commonKeysMatch2(a map[string]interface{}, b map[string]interface{}, m map[string][]interface{}) bool {
	for aKey, aVal := range a {
		_, useKey := m[aKey]
		if bVal, ok := b[aKey]; useKey && ok && !reflect.DeepEqual(aVal, bVal) {
			return false
		}
	}
	return true
}

// JobType describes what type of job we are about to run
type JobType int

const (
	// JobTypeDefault is all jobs that have a `run` attribute
	JobTypeDefault JobType = iota

	// JobTypeReusableWorkflowLocal is all jobs that have a `uses` that is a local workflow in the .github/workflows directory
	JobTypeReusableWorkflowLocal

	// JobTypeReusableWorkflowRemote is all jobs that have a `uses` that references a workflow file in a github repo
	JobTypeReusableWorkflowRemote

	// JobTypeInvalid represents a job which is not configured correctly
	JobTypeInvalid
)

func (j JobType) String() string {
	switch j {
	case JobTypeDefault:
		return "default"
	case JobTypeReusableWorkflowLocal:
		return "local-reusable-workflow"
	case JobTypeReusableWorkflowRemote:
		return "remote-reusable-workflow"
	}
	return "unknown"
}

// Type returns the type of the job
func (j *Job) Type() (JobType, error) {
	isReusable := j.Uses != ""

	if isReusable {
		isYaml, _ := regexp.MatchString(`\.(ya?ml)(?:$|@)`, j.Uses)

		if isYaml {
			isLocalPath := strings.HasPrefix(j.Uses, "./")
			isRemotePath, _ := regexp.MatchString(`^[^.](.+?/){2,}.+\.ya?ml@`, j.Uses)
			hasVersion, _ := regexp.MatchString(`\.ya?ml@`, j.Uses)

			if isLocalPath {
				return JobTypeReusableWorkflowLocal, nil
			} else if isRemotePath && hasVersion {
				return JobTypeReusableWorkflowRemote, nil
			}
		}

		return JobTypeInvalid, fmt.Errorf("`uses` key references invalid workflow path '%s'. Must start with './' if it's a local workflow, or must start with '<org>/<repo>/' and include an '@' if it's a remote workflow", j.Uses)
	}

	return JobTypeDefault, nil
}

// ContainerSpec is the specification of the container to use for the job
type ContainerSpec struct {
	Image       string            `yaml:"image"`
	Env         map[string]string `yaml:"env"`
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	Options     string            `yaml:"options"`
	Credentials map[string]string `yaml:"credentials"`
	Entrypoint  string
	Args        string
	Name        string
	Reuse       bool
}

// Step is the structure of one step in a job
type Step struct {
	ID                 string            `yaml:"id"`
	If                 yaml.Node         `yaml:"if"`
	Name               string            `yaml:"name"`
	Uses               string            `yaml:"uses"`
	Run                string            `yaml:"run"`
	WorkingDirectory   string            `yaml:"working-directory"`
	Shell              string            `yaml:"shell"`
	Env                yaml.Node         `yaml:"env"`
	With               map[string]string `yaml:"with"`
	RawContinueOnError string            `yaml:"continue-on-error"`
	TimeoutMinutes     string            `yaml:"timeout-minutes"`
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

// Environment returns string-based key=value map for a step
func (s *Step) Environment() map[string]string {
	return environment(s.Env)
}

// GetEnv gets the env for a step
func (s *Step) GetEnv() map[string]string {
	env := s.Environment()

	for k, v := range s.With {
		envKey := regexp.MustCompile("[^A-Z0-9-]").ReplaceAllString(strings.ToUpper(k), "_")
		envKey = fmt.Sprintf("INPUT_%s", strings.ToUpper(envKey))
		env[envKey] = v
	}
	return env
}

// ShellCommand returns the command for the shell
func (s *Step) ShellCommand() string {
	shellCommand := ""

	//Reference: https://github.com/actions/runner/blob/8109c962f09d9acc473d92c595ff43afceddb347/src/Runner.Worker/Handlers/ScriptHandlerHelpers.cs#L9-L17
	switch s.Shell {
	case "", "bash":
		shellCommand = "bash --noprofile --norc -e -o pipefail {0}"
	case "pwsh":
		shellCommand = "pwsh -command . '{0}'"
	case "python":
		shellCommand = "python {0}"
	case "sh":
		shellCommand = "sh -e {0}"
	case "cmd":
		shellCommand = "cmd /D /E:ON /V:OFF /S /C \"CALL \"{0}\"\""
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

	// StepTypeReusableWorkflowLocal is all steps that have a `uses` that is a local workflow in the .github/workflows directory
	StepTypeReusableWorkflowLocal

	// StepTypeReusableWorkflowRemote is all steps that have a `uses` that references a workflow file in a github repo
	StepTypeReusableWorkflowRemote

	// StepTypeInvalid is for steps that have invalid step action
	StepTypeInvalid
)

func (s StepType) String() string {
	switch s {
	case StepTypeInvalid:
		return "invalid"
	case StepTypeRun:
		return "run"
	case StepTypeUsesActionLocal:
		return "local-action"
	case StepTypeUsesActionRemote:
		return "remote-action"
	case StepTypeUsesDockerURL:
		return "docker"
	case StepTypeReusableWorkflowLocal:
		return "local-reusable-workflow"
	case StepTypeReusableWorkflowRemote:
		return "remote-reusable-workflow"
	}
	return "unknown"
}

// Type returns the type of the step
func (s *Step) Type() StepType {
	if s.Run == "" && s.Uses == "" {
		return StepTypeInvalid
	}

	if s.Run != "" {
		if s.Uses != "" {
			return StepTypeInvalid
		}
		return StepTypeRun
	} else if strings.HasPrefix(s.Uses, "docker://") {
		return StepTypeUsesDockerURL
	} else if strings.HasPrefix(s.Uses, "./.github/workflows") && (strings.HasSuffix(s.Uses, ".yml") || strings.HasSuffix(s.Uses, ".yaml")) {
		return StepTypeReusableWorkflowLocal
	} else if !strings.HasPrefix(s.Uses, "./") && strings.Contains(s.Uses, ".github/workflows") && (strings.Contains(s.Uses, ".yml@") || strings.Contains(s.Uses, ".yaml@")) {
		return StepTypeReusableWorkflowRemote
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
			if j.Name == "" {
				j.Name = id
			}
			if j.If.Value == "" {
				j.If.Value = "success()"
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

var OnDecodeNodeError = func(node yaml.Node, out interface{}, err error) {
	log.Fatalf("Failed to decode node %v into %T: %v", node, out, err)
}

func decodeNode(node yaml.Node, out interface{}) bool {
	if err := node.Decode(out); err != nil {
		if OnDecodeNodeError != nil {
			OnDecodeNodeError(node, out, err)
		}
		return false
	}
	return true
}
