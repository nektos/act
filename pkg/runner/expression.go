package runner

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/exprparser"
	log "github.com/sirupsen/logrus"
)

// ExpressionEvaluator is the interface for evaluating expressions
type ExpressionEvaluator interface {
	evaluate(string, bool) (interface{}, error)
	Interpolate(string) string
}

// NewExpressionEvaluator creates a new evaluator
func (rc *RunContext) NewExpressionEvaluator(context string) ExpressionEvaluator {
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
		// todo: should be unavailable in job context
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
		// todo: should be unavailable in step context
		// but required to interpolate/evaluate the inputs in actions/composite
		Inputs: rc.Inputs,
	}
	return expressionEvaluator{
		interpreter: exprparser.NewInterpeter(ee, exprparser.Config{
			Run:        rc.Run,
			WorkingDir: rc.Config.Workdir,
			Context:    context,
		}),
	}
}

type expressionEvaluator struct {
	interpreter exprparser.Interpreter
}

func (ee expressionEvaluator) evaluate(in string, isIfExpression bool) (interface{}, error) {
	evaluated, err := ee.interpreter.Evaluate(in, isIfExpression)
	return evaluated, err
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

	log.Debugf("expression '%s' evaluated to '%s'", expr, evaluated)

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
		return false, fmt.Errorf("Unable to map return type to boolean for '%s'", expr)
	}

	log.Debugf("expression '%s' evaluated to '%t'", nextExpr, result)

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
