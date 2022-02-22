package runner

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/exprparser"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ExpressionEvaluator is the interface for evaluating expressions
type ExpressionEvaluator interface {
	evaluate(string, bool) (interface{}, error)
	EvaluateYamlNode(node *yaml.Node) error
	Interpolate(string) string
}

// NewExpressionEvaluator creates a new evaluator
func (rc *RunContext) NewExpressionEvaluator() ExpressionEvaluator {
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

	secrets := rc.Config.Secrets
	if rc.Composite != nil {
		secrets = nil
	}

	ee := &exprparser.EvaluationEnvironment{
		Github: rc.getGithubContext(),
		Env:    rc.GetEnv(),
		Job:    rc.getJobContext(),
		// todo: should be unavailable
		// but required to interpolate/evaluate the step outputs on the job
		Steps: rc.getStepsContext(),
		Runner: map[string]interface{}{
			"os":         "Linux",
			"temp":       "/tmp",
			"tool_cache": "/opt/hostedtoolcache",
		},
		Secrets:  secrets,
		Strategy: strategy,
		Matrix:   rc.Matrix,
		Needs:    using,
		Inputs:   rc.Inputs,
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
func (sc *StepContext) NewExpressionEvaluator() ExpressionEvaluator {
	rc := sc.RunContext
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

	secrets := rc.Config.Secrets
	if rc.Composite != nil {
		secrets = nil
	}

	ee := &exprparser.EvaluationEnvironment{
		Github: rc.getGithubContext(),
		Env:    rc.GetEnv(),
		Job:    rc.getJobContext(),
		Steps:  rc.getStepsContext(),
		Runner: map[string]interface{}{
			"os":         "Linux",
			"temp":       "/tmp",
			"tool_cache": "/opt/hostedtoolcache",
		},
		Secrets:  secrets,
		Strategy: strategy,
		Matrix:   rc.Matrix,
		Needs:    using,
		// todo: should be unavailable
		// but required to interpolate/evaluate the inputs in actions/composite
		Inputs: rc.Inputs,
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

func (ee expressionEvaluator) evaluate(in string, isIfExpression bool) (interface{}, error) {
	log.Debugf("evaluating expression '%s'", in)
	evaluated, err := ee.interpreter.Evaluate(in, isIfExpression)
	log.Debugf("expression '%s' evaluated to '%t'", in, evaluated)
	return evaluated, err
}

func (ee expressionEvaluator) evaluateScalarYamlNode(node *yaml.Node) error {
	var in string
	if err := node.Decode(&in); err != nil {
		return err
	}
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return nil
	}
	expr, _ := rewriteSubExpression(in, false)
	if in != expr {
		log.Debugf("expression '%s' rewritten to '%s'", in, expr)
	}
	res, err := ee.evaluate(expr, false)
	if err != nil {
		return err
	}
	return node.Encode(res)
}

func (ee expressionEvaluator) evaluateMappingYamlNode(node *yaml.Node) error {
	// GitHub has this undocumented feature to merge maps, called insert directive
	insertDirective := regexp.MustCompile(`\${{\s*insert\s*}}`)
	for i := 0; i < len(node.Content)/2; {
		k := node.Content[i*2]
		v := node.Content[i*2+1]
		if err := ee.EvaluateYamlNode(v); err != nil {
			return err
		}
		var sk string
		// Merge the nested map of the insert directive
		if k.Decode(&sk) == nil && insertDirective.MatchString(sk) {
			node.Content = append(append(node.Content[:i*2], v.Content...), node.Content[(i+1)*2:]...)
			i += len(v.Content) / 2
		} else {
			if err := ee.EvaluateYamlNode(k); err != nil {
				return err
			}
			i++
		}
	}
	return nil
}

func (ee expressionEvaluator) evaluateSequenceYamlNode(node *yaml.Node) error {
	for i := 0; i < len(node.Content); {
		v := node.Content[i]
		// Preserve nested sequences
		wasseq := v.Kind == yaml.SequenceNode
		if err := ee.EvaluateYamlNode(v); err != nil {
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

func (ee expressionEvaluator) EvaluateYamlNode(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return ee.evaluateScalarYamlNode(node)
	case yaml.MappingNode:
		return ee.evaluateMappingYamlNode(node)
	case yaml.SequenceNode:
		return ee.evaluateSequenceYamlNode(node)
	default:
		return nil
	}
}

func (ee expressionEvaluator) Interpolate(in string) string {
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return in
	}

	expr, _ := rewriteSubExpression(in, true)
	if in != expr {
		log.Debugf("expression '%s' rewritten to '%s'", in, expr)
	}

	evaluated, err := ee.evaluate(expr, false)
	if err != nil {
		log.Errorf("Unable to interpolate expression '%s': %s", expr, err)
		return ""
	}

	value, ok := evaluated.(string)
	if !ok {
		panic(fmt.Sprintf("Expression %s did not evaluate to a string", expr))
	}

	return value
}

// EvalBool evaluates an expression against given evaluator
func EvalBool(evaluator ExpressionEvaluator, expr string) (bool, error) {
	nextExpr, _ := rewriteSubExpression(expr, false)
	if expr != nextExpr {
		log.Debugf("expression '%s' rewritten to '%s'", expr, nextExpr)
	}

	evaluated, err := evaluator.evaluate(nextExpr, true)
	if err != nil {
		return false, err
	}

	var result bool

	switch t := evaluated.(type) {
	case bool:
		result = t
	case string:
		result = t != ""
	case int:
		result = t != 0
	case float64:
		if math.IsNaN(t) {
			result = false
		} else {
			result = t != 0
		}
	default:
		return false, fmt.Errorf("Unable to map type '%t' to boolean for '%s'", evaluated, expr)
	}

	return result, nil
}

func escapeFormatString(in string) string {
	return strings.ReplaceAll(strings.ReplaceAll(in, "{", "{{"), "}", "}}")
}

//nolint:gocyclo
func rewriteSubExpression(in string, forceFormat bool) (string, error) {
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

	return fmt.Sprintf("format('%s', %s)", strings.ReplaceAll(formatOut, "'", "''"), strings.Join(results, ", ")), nil
}
