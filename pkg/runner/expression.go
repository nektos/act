package runner

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"strings"
	"time"

	_ "embed"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/exprparser"
	"github.com/nektos/act/pkg/model"
	"gopkg.in/yaml.v3"
)

// ExpressionEvaluator is the interface for evaluating expressions
type ExpressionEvaluator interface {
	evaluate(context.Context, string, exprparser.DefaultStatusCheck) (interface{}, error)
	EvaluateYamlNode(context.Context, *yaml.Node) error
	Interpolate(context.Context, string) string
}

// NewExpressionEvaluator creates a new evaluator
func (rc *RunContext) NewExpressionEvaluator(ctx context.Context) ExpressionEvaluator {
	return rc.NewExpressionEvaluatorWithEnv(ctx, rc.GetEnv())
}

func (rc *RunContext) NewExpressionEvaluatorWithEnv(ctx context.Context, env map[string]string) ExpressionEvaluator {
	var workflowCallResult map[string]*model.WorkflowCallResult

	// todo: cleanup EvaluationEnvironment creation
	using := make(map[string]exprparser.Needs)
	strategy := make(map[string]interface{})
	if rc.Run != nil {
		job := rc.Run.Job()
		if job != nil && job.Strategy != nil {
			strategy["fail-fast"] = job.Strategy.FailFast
			strategy["max-parallel"] = job.Strategy.MaxParallel
		}

		jobs := rc.Run.Workflow.Jobs
		jobNeeds := rc.Run.Job().Needs()

		for _, needs := range jobNeeds {
			using[needs] = exprparser.Needs{
				Outputs: jobs[needs].Outputs,
				Result:  jobs[needs].Result,
			}
		}

		// only setup jobs context in case of workflow_call
		// and existing expression evaluator (this means, jobs are at
		// least ready to run)
		if rc.caller != nil && rc.ExprEval != nil {
			workflowCallResult = map[string]*model.WorkflowCallResult{}

			for jobName, job := range jobs {
				result := model.WorkflowCallResult{
					Outputs: map[string]string{},
				}
				for k, v := range job.Outputs {
					result.Outputs[k] = v
				}
				workflowCallResult[jobName] = &result
			}
		}
	}

	ghc := rc.getGithubContext(ctx)
	inputs := getEvaluatorInputs(ctx, rc, nil, ghc)

	ee := &exprparser.EvaluationEnvironment{
		Github: ghc,
		Env:    env,
		Job:    rc.getJobContext(),
		Jobs:   &workflowCallResult,
		// todo: should be unavailable
		// but required to interpolate/evaluate the step outputs on the job
		Steps:     rc.getStepsContext(),
		Secrets:   getWorkflowSecrets(ctx, rc),
		Vars:      getWorkflowVars(ctx, rc),
		Strategy:  strategy,
		Matrix:    rc.Matrix,
		Needs:     using,
		Inputs:    inputs,
		HashFiles: getHashFilesFunction(ctx, rc),
	}
	if rc.JobContainer != nil {
		ee.Runner = rc.JobContainer.GetRunnerContext(ctx)
	}
	return expressionEvaluator{
		interpreter: exprparser.NewInterpeter(ee, exprparser.Config{
			Run:        rc.Run,
			WorkingDir: rc.Config.Workdir,
			Context:    "job",
		}),
	}
}

//go:embed hashfiles/index.js
var hashfiles string

// NewExpressionEvaluator creates a new evaluator
func (rc *RunContext) NewStepExpressionEvaluator(ctx context.Context, step step) ExpressionEvaluator {
	// todo: cleanup EvaluationEnvironment creation
	job := rc.Run.Job()
	strategy := make(map[string]interface{})
	if job.Strategy != nil {
		strategy["fail-fast"] = job.Strategy.FailFast
		strategy["max-parallel"] = job.Strategy.MaxParallel
	}

	jobs := rc.Run.Workflow.Jobs
	jobNeeds := rc.Run.Job().Needs()

	using := make(map[string]exprparser.Needs)
	for _, needs := range jobNeeds {
		using[needs] = exprparser.Needs{
			Outputs: jobs[needs].Outputs,
			Result:  jobs[needs].Result,
		}
	}

	ghc := rc.getGithubContext(ctx)
	inputs := getEvaluatorInputs(ctx, rc, step, ghc)

	ee := &exprparser.EvaluationEnvironment{
		Github:   step.getGithubContext(ctx),
		Env:      *step.getEnv(),
		Job:      rc.getJobContext(),
		Steps:    rc.getStepsContext(),
		Secrets:  getWorkflowSecrets(ctx, rc),
		Vars:     getWorkflowVars(ctx, rc),
		Strategy: strategy,
		Matrix:   rc.Matrix,
		Needs:    using,
		// todo: should be unavailable
		// but required to interpolate/evaluate the inputs in actions/composite
		Inputs:    inputs,
		HashFiles: getHashFilesFunction(ctx, rc),
	}
	if rc.JobContainer != nil {
		ee.Runner = rc.JobContainer.GetRunnerContext(ctx)
	}
	return expressionEvaluator{
		interpreter: exprparser.NewInterpeter(ee, exprparser.Config{
			Run:        rc.Run,
			WorkingDir: rc.Config.Workdir,
			Context:    "step",
		}),
	}
}

func getHashFilesFunction(ctx context.Context, rc *RunContext) func(v []reflect.Value) (interface{}, error) {
	hashFiles := func(v []reflect.Value) (interface{}, error) {
		if rc.JobContainer != nil {
			timeed, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			name := "workflow/hashfiles/index.js"
			hout := &bytes.Buffer{}
			herr := &bytes.Buffer{}
			patterns := []string{}
			followSymlink := false

			for i, p := range v {
				s := p.String()
				if i == 0 {
					if strings.HasPrefix(s, "--") {
						if strings.EqualFold(s, "--follow-symbolic-links") {
							followSymlink = true
							continue
						}
						return "", fmt.Errorf("Invalid glob option %s, available option: '--follow-symbolic-links'", s)
					}
				}
				patterns = append(patterns, s)
			}
			env := map[string]string{}
			for k, v := range rc.Env {
				env[k] = v
			}
			env["patterns"] = strings.Join(patterns, "\n")
			if followSymlink {
				env["followSymbolicLinks"] = "true"
			}

			stdout, stderr := rc.JobContainer.ReplaceLogWriter(hout, herr)
			_ = rc.JobContainer.Copy(rc.JobContainer.GetActPath(), &container.FileEntry{
				Name: name,
				Mode: 0o644,
				Body: hashfiles,
			}).
				Then(rc.execJobContainer([]string{"node", path.Join(rc.JobContainer.GetActPath(), name)},
					env, "", "")).
				Finally(func(context.Context) error {
					rc.JobContainer.ReplaceLogWriter(stdout, stderr)
					return nil
				})(timeed)
			output := hout.String() + "\n" + herr.String()
			guard := "__OUTPUT__"
			outstart := strings.Index(output, guard)
			if outstart != -1 {
				outstart += len(guard)
				outend := strings.Index(output[outstart:], guard)
				if outend != -1 {
					return output[outstart : outstart+outend], nil
				}
			}
		}
		return "", nil
	}
	return hashFiles
}

type expressionEvaluator struct {
	interpreter exprparser.Interpreter
}

func (ee expressionEvaluator) evaluate(ctx context.Context, in string, defaultStatusCheck exprparser.DefaultStatusCheck) (interface{}, error) {
	logger := common.Logger(ctx)
	logger.Debugf("evaluating expression '%s'", in)
	evaluated, err := ee.interpreter.Evaluate(in, defaultStatusCheck)

	printable := regexp.MustCompile(`::add-mask::.*`).ReplaceAllString(fmt.Sprintf("%t", evaluated), "::add-mask::***)")
	logger.Debugf("expression '%s' evaluated to '%s'", in, printable)

	return evaluated, err
}

func (ee expressionEvaluator) evaluateScalarYamlNode(ctx context.Context, node *yaml.Node) (*yaml.Node, error) {
	var in string
	if err := node.Decode(&in); err != nil {
		return nil, err
	}
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return nil, nil
	}
	expr, _ := rewriteSubExpression(ctx, in, false)
	res, err := ee.evaluate(ctx, expr, exprparser.DefaultStatusCheckNone)
	if err != nil {
		return nil, err
	}
	ret := &yaml.Node{}
	if err := ret.Encode(res); err != nil {
		return nil, err
	}
	return ret, err
}

func (ee expressionEvaluator) evaluateMappingYamlNode(ctx context.Context, node *yaml.Node) (*yaml.Node, error) {
	var ret *yaml.Node
	// GitHub has this undocumented feature to merge maps, called insert directive
	insertDirective := regexp.MustCompile(`\${{\s*insert\s*}}`)
	for i := 0; i < len(node.Content)/2; i++ {
		changed := func() error {
			if ret == nil {
				ret = &yaml.Node{}
				if err := ret.Encode(node); err != nil {
					return err
				}
				ret.Content = ret.Content[:i*2]
			}
			return nil
		}
		k := node.Content[i*2]
		v := node.Content[i*2+1]
		ev, err := ee.evaluateYamlNodeInternal(ctx, v)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			if err := changed(); err != nil {
				return nil, err
			}
		} else {
			ev = v
		}
		var sk string
		// Merge the nested map of the insert directive
		if k.Decode(&sk) == nil && insertDirective.MatchString(sk) {
			if ev.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("failed to insert node %v into mapping %v unexpected type %v expected MappingNode", ev, node, ev.Kind)
			}
			if err := changed(); err != nil {
				return nil, err
			}
			ret.Content = append(ret.Content, ev.Content...)
		} else {
			ek, err := ee.evaluateYamlNodeInternal(ctx, k)
			if err != nil {
				return nil, err
			}
			if ek != nil {
				if err := changed(); err != nil {
					return nil, err
				}
			} else {
				ek = k
			}
			if ret != nil {
				ret.Content = append(ret.Content, ek, ev)
			}
		}
	}
	return ret, nil
}

func (ee expressionEvaluator) evaluateSequenceYamlNode(ctx context.Context, node *yaml.Node) (*yaml.Node, error) {
	var ret *yaml.Node
	for i := 0; i < len(node.Content); i++ {
		v := node.Content[i]
		// Preserve nested sequences
		wasseq := v.Kind == yaml.SequenceNode
		ev, err := ee.evaluateYamlNodeInternal(ctx, v)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			if ret == nil {
				ret = &yaml.Node{}
				if err := ret.Encode(node); err != nil {
					return nil, err
				}
				ret.Content = ret.Content[:i]
			}
			// GitHub has this undocumented feature to merge sequences / arrays
			// We have a nested sequence via evaluation, merge the arrays
			if ev.Kind == yaml.SequenceNode && !wasseq {
				ret.Content = append(ret.Content, ev.Content...)
			} else {
				ret.Content = append(ret.Content, ev)
			}
		} else if ret != nil {
			ret.Content = append(ret.Content, v)
		}
	}
	return ret, nil
}

func (ee expressionEvaluator) evaluateYamlNodeInternal(ctx context.Context, node *yaml.Node) (*yaml.Node, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return ee.evaluateScalarYamlNode(ctx, node)
	case yaml.MappingNode:
		return ee.evaluateMappingYamlNode(ctx, node)
	case yaml.SequenceNode:
		return ee.evaluateSequenceYamlNode(ctx, node)
	default:
		return nil, nil
	}
}

func (ee expressionEvaluator) EvaluateYamlNode(ctx context.Context, node *yaml.Node) error {
	ret, err := ee.evaluateYamlNodeInternal(ctx, node)
	if err != nil {
		return err
	}
	if ret != nil {
		return ret.Decode(node)
	}
	return nil
}

func (ee expressionEvaluator) Interpolate(ctx context.Context, in string) string {
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return in
	}

	expr, _ := rewriteSubExpression(ctx, in, true)
	evaluated, err := ee.evaluate(ctx, expr, exprparser.DefaultStatusCheckNone)
	if err != nil {
		common.Logger(ctx).Errorf("Unable to interpolate expression '%s': %s", expr, err)
		return ""
	}

	value, ok := evaluated.(string)
	if !ok {
		panic(fmt.Sprintf("Expression %s did not evaluate to a string", expr))
	}

	return value
}

// EvalBool evaluates an expression against given evaluator
func EvalBool(ctx context.Context, evaluator ExpressionEvaluator, expr string, defaultStatusCheck exprparser.DefaultStatusCheck) (bool, error) {
	nextExpr, _ := rewriteSubExpression(ctx, expr, false)

	evaluated, err := evaluator.evaluate(ctx, nextExpr, defaultStatusCheck)
	if err != nil {
		return false, err
	}

	return exprparser.IsTruthy(evaluated), nil
}

func escapeFormatString(in string) string {
	return strings.ReplaceAll(strings.ReplaceAll(in, "{", "{{"), "}", "}}")
}

//nolint:gocyclo
func rewriteSubExpression(ctx context.Context, in string, forceFormat bool) (string, error) {
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return in, nil
	}

	strPattern := regexp.MustCompile("(?:''|[^'])*'")
	pos := 0
	exprStart := -1
	strStart := -1
	var results []string
	formatOut := ""
	for pos < len(in) {
		if strStart > -1 {
			matches := strPattern.FindStringIndex(in[pos:])
			if matches == nil {
				panic("unclosed string.")
			}

			strStart = -1
			pos += matches[1]
		} else if exprStart > -1 {
			exprEnd := strings.Index(in[pos:], "}}")
			strStart = strings.Index(in[pos:], "'")

			if exprEnd > -1 && strStart > -1 {
				if exprEnd < strStart {
					strStart = -1
				} else {
					exprEnd = -1
				}
			}

			if exprEnd > -1 {
				formatOut += fmt.Sprintf("{%d}", len(results))
				results = append(results, strings.TrimSpace(in[exprStart:pos+exprEnd]))
				pos += exprEnd + 2
				exprStart = -1
			} else if strStart > -1 {
				pos += strStart + 1
			} else {
				panic("unclosed expression.")
			}
		} else {
			exprStart = strings.Index(in[pos:], "${{")
			if exprStart != -1 {
				formatOut += escapeFormatString(in[pos : pos+exprStart])
				exprStart = pos + exprStart + 3
				pos = exprStart
			} else {
				formatOut += escapeFormatString(in[pos:])
				pos = len(in)
			}
		}
	}

	if len(results) == 1 && formatOut == "{0}" && !forceFormat {
		return in, nil
	}

	out := fmt.Sprintf("format('%s', %s)", strings.ReplaceAll(formatOut, "'", "''"), strings.Join(results, ", "))
	if in != out {
		common.Logger(ctx).Debugf("expression '%s' rewritten to '%s'", in, out)
	}
	return out, nil
}

//nolint:gocyclo
func getEvaluatorInputs(ctx context.Context, rc *RunContext, step step, ghc *model.GithubContext) map[string]interface{} {
	inputs := map[string]interface{}{}

	setupWorkflowInputs(ctx, &inputs, rc)

	var env map[string]string
	if step != nil {
		env = *step.getEnv()
	} else {
		env = rc.GetEnv()
	}

	for k, v := range env {
		if strings.HasPrefix(k, "INPUT_") {
			inputs[strings.ToLower(strings.TrimPrefix(k, "INPUT_"))] = v
		}
	}

	if ghc.EventName == "workflow_dispatch" {
		config := rc.Run.Workflow.WorkflowDispatchConfig()
		if config != nil && config.Inputs != nil {
			for k, v := range config.Inputs {
				value := nestedMapLookup(ghc.Event, "inputs", k)
				if value == nil {
					value = v.Default
				}
				if v.Type == "boolean" {
					inputs[k] = value == "true"
				} else {
					inputs[k] = value
				}
			}
		}
	}

	if ghc.EventName == "workflow_call" {
		config := rc.Run.Workflow.WorkflowCallConfig()
		if config != nil && config.Inputs != nil {
			for k, v := range config.Inputs {
				value := nestedMapLookup(ghc.Event, "inputs", k)
				if value == nil {
					value = v.Default
				}
				if v.Type == "boolean" {
					inputs[k] = value == "true"
				} else {
					inputs[k] = value
				}
			}
		}
	}
	return inputs
}

func setupWorkflowInputs(ctx context.Context, inputs *map[string]interface{}, rc *RunContext) {
	if rc.caller != nil {
		config := rc.Run.Workflow.WorkflowCallConfig()

		for name, input := range config.Inputs {
			value := rc.caller.runContext.Run.Job().With[name]
			if value != nil {
				if str, ok := value.(string); ok {
					// evaluate using the calling RunContext (outside)
					value = rc.caller.runContext.ExprEval.Interpolate(ctx, str)
				}
			}

			if value == nil && config != nil && config.Inputs != nil {
				value = input.Default
				if rc.ExprEval != nil {
					if str, ok := value.(string); ok {
						// evaluate using the called RunContext (inside)
						value = rc.ExprEval.Interpolate(ctx, str)
					}
				}
			}

			(*inputs)[name] = value
		}
	}
}

func getWorkflowSecrets(ctx context.Context, rc *RunContext) map[string]string {
	if rc.caller != nil {
		job := rc.caller.runContext.Run.Job()
		secrets := job.Secrets()

		if secrets == nil && job.InheritSecrets() {
			secrets = rc.caller.runContext.Config.Secrets
		}

		if secrets == nil {
			secrets = map[string]string{}
		}

		for k, v := range secrets {
			secrets[k] = rc.caller.runContext.ExprEval.Interpolate(ctx, v)
		}

		return secrets
	}

	return rc.Config.Secrets
}

func getWorkflowVars(_ context.Context, rc *RunContext) map[string]string {
	return rc.Config.Vars
}
