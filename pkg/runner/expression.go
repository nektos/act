package runner

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/common"
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
	// todo: cleanup EvaluationEnvironment creation
	using := make(map[string]map[string]map[string]string)
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
			using[needs] = map[string]map[string]string{
				"outputs": jobs[needs].Outputs,
			}
		}
	}

	ghc := rc.getGithubContext(ctx)
	inputs := getEvaluatorInputs(ctx, rc, nil, ghc)

	ee := &exprparser.EvaluationEnvironment{
		Github: ghc,
		Env:    env,
		Job:    rc.getJobContext(),
		// todo: should be unavailable
		// but required to interpolate/evaluate the step outputs on the job
		Steps:    rc.getStepsContext(),
		Secrets:  rc.Config.Secrets,
		Strategy: strategy,
		Matrix:   rc.Matrix,
		Needs:    using,
		Inputs:   inputs,
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

	using := make(map[string]map[string]map[string]string)
	for _, needs := range jobNeeds {
		using[needs] = map[string]map[string]string{
			"outputs": jobs[needs].Outputs,
		}
	}

	ghc := rc.getGithubContext(ctx)
	inputs := getEvaluatorInputs(ctx, rc, step, ghc)

	ee := &exprparser.EvaluationEnvironment{
		Github:   step.getGithubContext(ctx),
		Env:      *step.getEnv(),
		Job:      rc.getJobContext(),
		Steps:    rc.getStepsContext(),
		Secrets:  rc.Config.Secrets,
		Strategy: strategy,
		Matrix:   rc.Matrix,
		Needs:    using,
		// todo: should be unavailable
		// but required to interpolate/evaluate the inputs in actions/composite
		Inputs: inputs,
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

func (ee expressionEvaluator) evaluateScalarYamlNode(ctx context.Context, node *yaml.Node) error {
	var in string
	if err := node.Decode(&in); err != nil {
		return err
	}
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return nil
	}
	expr, _ := rewriteSubExpression(ctx, in, false)
	res, err := ee.evaluate(ctx, expr, exprparser.DefaultStatusCheckNone)
	if err != nil {
		return err
	}
	return node.Encode(res)
}

func (ee expressionEvaluator) evaluateMappingYamlNode(ctx context.Context, node *yaml.Node) error {
	// GitHub has this undocumented feature to merge maps, called insert directive
	insertDirective := regexp.MustCompile(`\${{\s*insert\s*}}`)
	for i := 0; i < len(node.Content)/2; {
		k := node.Content[i*2]
		v := node.Content[i*2+1]
		if err := ee.EvaluateYamlNode(ctx, v); err != nil {
			return err
		}
		var sk string
		// Merge the nested map of the insert directive
		if k.Decode(&sk) == nil && insertDirective.MatchString(sk) {
			node.Content = append(append(node.Content[:i*2], v.Content...), node.Content[(i+1)*2:]...)
			i += len(v.Content) / 2
		} else {
			if err := ee.EvaluateYamlNode(ctx, k); err != nil {
				return err
			}
			i++
		}
	}
	return nil
}

func (ee expressionEvaluator) evaluateSequenceYamlNode(ctx context.Context, node *yaml.Node) error {
	for i := 0; i < len(node.Content); {
		v := node.Content[i]
		// Preserve nested sequences
		wasseq := v.Kind == yaml.SequenceNode
		if err := ee.EvaluateYamlNode(ctx, v); err != nil {
			return err
		}
		// GitHub has this undocumented feature to merge sequences / arrays
		// We have a nested sequence via evaluation, merge the arrays
		if v.Kind == yaml.SequenceNode && !wasseq {
			node.Content = append(append(node.Content[:i], v.Content...), node.Content[i+1:]...)
			i += len(v.Content)
		} else {
			i++
		}
	}
	return nil
}

func (ee expressionEvaluator) EvaluateYamlNode(ctx context.Context, node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return ee.evaluateScalarYamlNode(ctx, node)
	case yaml.MappingNode:
		return ee.evaluateMappingYamlNode(ctx, node)
	case yaml.SequenceNode:
		return ee.evaluateSequenceYamlNode(ctx, node)
	default:
		return nil
	}
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

func getEvaluatorInputs(ctx context.Context, rc *RunContext, step step, ghc *model.GithubContext) map[string]interface{} {
	inputs := map[string]interface{}{}

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

	return inputs
}
