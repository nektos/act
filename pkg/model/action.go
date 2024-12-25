package model

import (
	"fmt"
	"io"
	"strings"

	"github.com/nektos/act/pkg/schema"
	"gopkg.in/yaml.v3"
)

// ActionRunsUsing is the type of runner for the action
type ActionRunsUsing string

func (a *ActionRunsUsing) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var using string
	if err := unmarshal(&using); err != nil {
		return err
	}

	// Force input to lowercase for case insensitive comparison
	format := ActionRunsUsing(strings.ToLower(using))
	switch format {
	case ActionRunsUsingNode20, ActionRunsUsingNode16, ActionRunsUsingNode12, ActionRunsUsingDocker, ActionRunsUsingComposite:
		*a = format
	default:
		return fmt.Errorf("The runs.using key in action.yml must be one of: %v, got %s", []string{
			ActionRunsUsingComposite,
			ActionRunsUsingDocker,
			ActionRunsUsingNode12,
			ActionRunsUsingNode16,
			ActionRunsUsingNode20,
		}, format)
	}
	return nil
}

const (
	// ActionRunsUsingNode12 for running with node12
	ActionRunsUsingNode12 = "node12"
	// ActionRunsUsingNode16 for running with node16
	ActionRunsUsingNode16 = "node16"
	// ActionRunsUsingNode20 for running with node20
	ActionRunsUsingNode20 = "node20"
	// ActionRunsUsingDocker for running with docker
	ActionRunsUsingDocker = "docker"
	// ActionRunsUsingComposite for running composite
	ActionRunsUsingComposite = "composite"
)

// ActionRuns are a field in Action
type ActionRuns struct {
	Using          ActionRunsUsing   `yaml:"using"`
	Env            map[string]string `yaml:"env"`
	Main           string            `yaml:"main"`
	Pre            string            `yaml:"pre"`
	PreIf          string            `yaml:"pre-if"`
	Post           string            `yaml:"post"`
	PostIf         string            `yaml:"post-if"`
	Image          string            `yaml:"image"`
	PreEntrypoint  string            `yaml:"pre-entrypoint"`
	Entrypoint     string            `yaml:"entrypoint"`
	PostEntrypoint string            `yaml:"post-entrypoint"`
	Args           []string          `yaml:"args"`
	Steps          []Step            `yaml:"steps"`
}

// Action describes a metadata file for GitHub actions. The metadata filename must be either action.yml or action.yaml. The data in the metadata file defines the inputs, outputs and main entrypoint for your action.
type Action struct {
	Name        string            `yaml:"name"`
	Author      string            `yaml:"author"`
	Description string            `yaml:"description"`
	Inputs      map[string]Input  `yaml:"inputs"`
	Outputs     map[string]Output `yaml:"outputs"`
	Runs        ActionRuns        `yaml:"runs"`
	Branding    struct {
		Color string `yaml:"color"`
		Icon  string `yaml:"icon"`
	} `yaml:"branding"`
}

func (a *Action) UnmarshalYAML(node *yaml.Node) error {
	// Validate the schema before deserializing it into our model
	if err := (&schema.Node{
		Definition: "action-root",
		Schema:     schema.GetActionSchema(),
	}).UnmarshalYAML(node); err != nil {
		return err
	}
	type ActionDefault Action
	return node.Decode((*ActionDefault)(a))
}

// Input parameters allow you to specify data that the action expects to use during runtime. GitHub stores input parameters as environment variables. Input ids with uppercase letters are converted to lowercase during runtime. We recommended using lowercase input ids.
type Input struct {
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default"`
}

// Output parameters allow you to declare data that an action sets. Actions that run later in a workflow can use the output data set in previously run actions. For example, if you had an action that performed the addition of two inputs (x + y = z), the action could output the sum (z) for other actions to use as an input.
type Output struct {
	Description string `yaml:"description"`
	Value       string `yaml:"value"`
}

// ReadAction reads an action from a reader
func ReadAction(in io.Reader) (*Action, error) {
	a := new(Action)
	err := yaml.NewDecoder(in).Decode(a)
	if err != nil {
		return nil, err
	}

	// set defaults
	if a.Runs.PreIf == "" {
		a.Runs.PreIf = "always()"
	}
	if a.Runs.PostIf == "" {
		a.Runs.PostIf = "always()"
	}

	return a, nil
}
