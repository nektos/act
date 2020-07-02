package runner

import (
	"fmt"
	"github.com/nektos/act/pkg/model"
	a "github.com/stretchr/testify/assert"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
)

func TestRunContext_EvalBool(t *testing.T) {
	hook := test.NewGlobal()
	assert := a.New(t)
	rc := &RunContext{
		Config: &Config{
			Workdir: ".",
		},
		Env: map[string]string{
			"TRUE":      "true",
			"FALSE":     "false",
			"SOME_TEXT": "text",
		},
		Run: &model.Run{
			JobID: "job1",
			Workflow: &model.Workflow{
				Name: "test-workflow",
				Jobs: map[string]*model.Job{
					"job1": {
						Strategy: &model.Strategy{
							Matrix: map[string][]interface{}{
								"os":  {"Linux", "Windows"},
								"foo": {"bar", "baz"},
							},
						},
					},
				},
			},
		},
		Matrix: map[string]interface{}{
			"os":  "Linux",
			"foo": "bar",
		},
		StepResults: map[string]*stepResult{
			"id1": {
				Outputs: map[string]string{
					"foo": "bar",
				},
				Success: true,
			},
		},
	}
	rc.ExprEval = rc.NewExpressionEvaluator()

	tables := []struct {
		in  string
		out bool
	}{
		// The basic ones
		{"true", true},
		{"false", false},
		{"1 !== 0", true},
		{"1 !== 1", false},
		{"1 == 0", false},
		{"1 == 1", true},
		{"1 > 2", false},
		{"1 < 2", true},
		{"success()", true},
		{"failure()", false},
		// And or
		{"true && false", false},
		{"true && 1 < 2", true},
		{"false || 1 < 2", true},
		{"false || false", false},
		// None boolable
		{"env.SOME_TEXT", false},
		{"env.UNKNOWN == 'true'", false},
		{"env.UNKNOWN", false},
		// Inline expressions
		{"env.TRUE == 'true'", true},
		{"env.FALSE == 'true'", false},
		{"env.TRUE", true},
		{"env.FALSE", false},
		{"!env.TRUE", false},
		{"!env.FALSE", true},
		{"${{ env.TRUE }}", true},
		{"${{ env.FALSE }}", false},
		{"${{ !env.TRUE }}", false},
		{"${{ !env.FALSE }}", true},
		{"!env.TRUE && true", false},
		{"!env.FALSE && true", true},
		{"!env.TRUE || true", true},
		{"!env.FALSE || false", true},
		{"${{env.TRUE == 'true'}}", true},
		{"${{env.FALSE == 'true'}}", false},
		{"${{env.FALSE == 'false'}}", true},
		// All together now
		{"false || env.TRUE == 'true'", true},
		{"true || env.FALSE == 'true'", true},
		{"true && env.TRUE == 'true'", true},
		{"false && env.TRUE == 'true'", false},
		{"env.FALSE == 'true' && env.TRUE == 'true'", false},
		{"env.FALSE == 'true' && true", false},
		{"${{env.FALSE == 'true'}} && true", false},
		// Check github context
		{"github.actor == 'nektos/act'", true},
		{"github.actor == 'unknown'", false},
	}

	for _, table := range tables {
		table := table
		t.Run(table.in, func(t *testing.T) {
			defer hook.Reset()
			b := rc.EvalBool(table.in)

			assert.Equal(table.out, b, fmt.Sprintf("Expected %s to be %v, was %v",table.in, table.out, b))
			assert.Empty(hook.LastEntry(), table.in)
		})
	}
}
