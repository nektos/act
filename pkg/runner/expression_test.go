package runner

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"testing"

	"github.com/nektos/act/pkg/exprparser"
	"github.com/nektos/act/pkg/model"
	assert "github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v3"
)

func createRunContext(t *testing.T) *RunContext {
	var yml yaml.Node
	err := yml.Encode(map[string][]interface{}{
		"os":  {"Linux", "Windows"},
		"foo": {"bar", "baz"},
	})
	assert.NoError(t, err)

	return &RunContext{
		Config: &Config{
			Workdir: ".",
			Secrets: map[string]string{
				"CASE_INSENSITIVE_SECRET": "value",
			},
			Vars: map[string]string{
				"CASE_INSENSITIVE_VAR": "value",
			},
		},
		Env: map[string]string{
			"key": "value",
		},
		Run: &model.Run{
			JobID: "job1",
			Workflow: &model.Workflow{
				Name: "test-workflow",
				Jobs: map[string]*model.Job{
					"job1": {
						Strategy: &model.Strategy{
							RawMatrix: yml,
						},
					},
				},
			},
		},
		Matrix: map[string]interface{}{
			"os":  "Linux",
			"foo": "bar",
		},
		StepResults: map[string]*model.StepResult{
			"idwithnothing": {
				Conclusion: model.StepStatusSuccess,
				Outcome:    model.StepStatusFailure,
				Outputs: map[string]string{
					"foowithnothing": "barwithnothing",
				},
			},
			"id-with-hyphens": {
				Conclusion: model.StepStatusSuccess,
				Outcome:    model.StepStatusFailure,
				Outputs: map[string]string{
					"foo-with-hyphens": "bar-with-hyphens",
				},
			},
			"id_with_underscores": {
				Conclusion: model.StepStatusSuccess,
				Outcome:    model.StepStatusFailure,
				Outputs: map[string]string{
					"foo_with_underscores": "bar_with_underscores",
				},
			},
		},
	}
}

func TestEvaluateRunContext(t *testing.T) {
	rc := createRunContext(t)
	ee := rc.NewExpressionEvaluator(context.Background())

	tables := []struct {
		in      string
		out     interface{}
		errMesg string
	}{
		{" 1 ", 1, ""},
		// {"1 + 3", "4", ""},
		// {"(1 + 3) * -2", "-8", ""},
		{"'my text'", "my text", ""},
		{"contains('my text', 'te')", true, ""},
		{"contains('my TEXT', 'te')", true, ""},
		{"contains(fromJSON('[\"my text\"]'), 'te')", false, ""},
		{"contains(fromJSON('[\"foo\",\"bar\"]'), 'bar')", true, ""},
		{"startsWith('hello world', 'He')", true, ""},
		{"endsWith('hello world', 'ld')", true, ""},
		{"format('0:{0} 2:{2} 1:{1}', 'zero', 'one', 'two')", "0:zero 2:two 1:one", ""},
		{"join(fromJSON('[\"hello\"]'),'octocat')", "hello", ""},
		{"join(fromJSON('[\"hello\",\"mona\",\"the\"]'),'octocat')", "hellooctocatmonaoctocatthe", ""},
		{"join('hello','mona')", "hello", ""},
		{"toJSON(env)", "{\n  \"ACT\": \"true\",\n  \"key\": \"value\"\n}", ""},
		{"toJson(env)", "{\n  \"ACT\": \"true\",\n  \"key\": \"value\"\n}", ""},
		{"(fromJSON('{\"foo\":\"bar\"}')).foo", "bar", ""},
		{"(fromJson('{\"foo\":\"bar\"}')).foo", "bar", ""},
		{"(fromJson('[\"foo\",\"bar\"]'))[1]", "bar", ""},
		// github does return an empty string for non-existent files
		{"hashFiles('**/non-extant-files')", "", ""},
		{"hashFiles('**/non-extant-files', '**/more-non-extant-files')", "", ""},
		{"hashFiles('**/non.extant.files')", "", ""},
		{"hashFiles('**/non''extant''files')", "", ""},
		{"success()", true, ""},
		{"failure()", false, ""},
		{"always()", true, ""},
		{"cancelled()", false, ""},
		{"github.workflow", "test-workflow", ""},
		{"github.actor", "nektos/act", ""},
		{"github.run_id", "1", ""},
		{"github.run_number", "1", ""},
		{"job.status", "success", ""},
		{"matrix.os", "Linux", ""},
		{"matrix.foo", "bar", ""},
		{"env.key", "value", ""},
		{"secrets.CASE_INSENSITIVE_SECRET", "value", ""},
		{"secrets.case_insensitive_secret", "value", ""},
		{"vars.CASE_INSENSITIVE_VAR", "value", ""},
		{"vars.case_insensitive_var", "value", ""},
		{"format('{{0}}', 'test')", "{0}", ""},
		{"format('{{{0}}}', 'test')", "{test}", ""},
		{"format('}}')", "}", ""},
		{"format('echo Hello {0} ${{Test}}', 'World')", "echo Hello World ${Test}", ""},
		{"format('echo Hello {0} ${{Test}}', github.undefined_property)", "echo Hello  ${Test}", ""},
		{"format('echo Hello {0}{1} ${{Te{0}st}}', github.undefined_property, 'World')", "echo Hello World ${Test}", ""},
		{"format('{0}', '{1}', 'World')", "{1}", ""},
		{"format('{{{0}', '{1}', 'World')", "{{1}", ""},
	}

	for _, table := range tables {
		t.Run(table.in, func(t *testing.T) {
			assertObject := assert.New(t)
			out, err := ee.evaluate(context.Background(), table.in, exprparser.DefaultStatusCheckNone)
			if table.errMesg == "" {
				assertObject.NoError(err, table.in)
				assertObject.Equal(table.out, out, table.in)
			} else {
				assertObject.Error(err, table.in)
				assertObject.Equal(table.errMesg, err.Error(), table.in)
			}
		})
	}
}

func TestEvaluateStep(t *testing.T) {
	rc := createRunContext(t)
	step := &stepRun{
		RunContext: rc,
	}

	ee := rc.NewStepExpressionEvaluator(context.Background(), step)

	tables := []struct {
		in      string
		out     interface{}
		errMesg string
	}{
		{"steps.idwithnothing.conclusion", model.StepStatusSuccess.String(), ""},
		{"steps.idwithnothing.outcome", model.StepStatusFailure.String(), ""},
		{"steps.idwithnothing.outputs.foowithnothing", "barwithnothing", ""},
		{"steps.id-with-hyphens.conclusion", model.StepStatusSuccess.String(), ""},
		{"steps.id-with-hyphens.outcome", model.StepStatusFailure.String(), ""},
		{"steps.id-with-hyphens.outputs.foo-with-hyphens", "bar-with-hyphens", ""},
		{"steps.id_with_underscores.conclusion", model.StepStatusSuccess.String(), ""},
		{"steps.id_with_underscores.outcome", model.StepStatusFailure.String(), ""},
		{"steps.id_with_underscores.outputs.foo_with_underscores", "bar_with_underscores", ""},
	}

	for _, table := range tables {
		t.Run(table.in, func(t *testing.T) {
			assertObject := assert.New(t)
			out, err := ee.evaluate(context.Background(), table.in, exprparser.DefaultStatusCheckNone)
			if table.errMesg == "" {
				assertObject.NoError(err, table.in)
				assertObject.Equal(table.out, out, table.in)
			} else {
				assertObject.Error(err, table.in)
				assertObject.Equal(table.errMesg, err.Error(), table.in)
			}
		})
	}
}

func TestInterpolate(t *testing.T) {
	rc := &RunContext{
		Config: &Config{
			Workdir: ".",
			Secrets: map[string]string{
				"CASE_INSENSITIVE_SECRET": "value",
			},
			Vars: map[string]string{
				"CASE_INSENSITIVE_VAR": "value",
			},
		},
		Env: map[string]string{
			"KEYWITHNOTHING":       "valuewithnothing",
			"KEY-WITH-HYPHENS":     "value-with-hyphens",
			"KEY_WITH_UNDERSCORES": "value_with_underscores",
			"SOMETHING_TRUE":       "true",
			"SOMETHING_FALSE":      "false",
		},
		Run: &model.Run{
			JobID: "job1",
			Workflow: &model.Workflow{
				Name: "test-workflow",
				Jobs: map[string]*model.Job{
					"job1": {},
				},
			},
		},
	}
	ee := rc.NewExpressionEvaluator(context.Background())
	tables := []struct {
		in  string
		out string
	}{
		{" text ", " text "},
		{" $text ", " $text "},
		{" ${text} ", " ${text} "},
		{" ${{          1                         }} to ${{2}} ", " 1 to 2 "},
		{" ${{  (true || false)  }} to ${{2}} ", " true to 2 "},
		{" ${{  (false   ||  '}}'  )    }} to ${{2}} ", " }} to 2 "},
		{" ${{ env.KEYWITHNOTHING }} ", " valuewithnothing "},
		{" ${{ env.KEY-WITH-HYPHENS }} ", " value-with-hyphens "},
		{" ${{ env.KEY_WITH_UNDERSCORES }} ", " value_with_underscores "},
		{"${{ secrets.CASE_INSENSITIVE_SECRET }}", "value"},
		{"${{ secrets.case_insensitive_secret }}", "value"},
		{"${{ vars.CASE_INSENSITIVE_VAR }}", "value"},
		{"${{ vars.case_insensitive_var }}", "value"},
		{"${{ env.UNKNOWN }}", ""},
		{"${{ env.SOMETHING_TRUE }}", "true"},
		{"${{ env.SOMETHING_FALSE }}", "false"},
		{"${{ !env.SOMETHING_TRUE }}", "false"},
		{"${{ !env.SOMETHING_FALSE }}", "false"},
		{"${{ !env.SOMETHING_TRUE && true }}", "false"},
		{"${{ !env.SOMETHING_FALSE && true }}", "false"},
		{"${{ env.SOMETHING_TRUE && true }}", "true"},
		{"${{ env.SOMETHING_FALSE && true }}", "true"},
		{"${{ !env.SOMETHING_TRUE || true }}", "true"},
		{"${{ !env.SOMETHING_FALSE || true }}", "true"},
		{"${{ !env.SOMETHING_TRUE && false }}", "false"},
		{"${{ !env.SOMETHING_FALSE && false }}", "false"},
		{"${{ !env.SOMETHING_TRUE || false }}", "false"},
		{"${{ !env.SOMETHING_FALSE || false }}", "false"},
		{"${{ env.SOMETHING_TRUE || false }}", "true"},
		{"${{ env.SOMETHING_FALSE || false }}", "false"},
		{"${{ env.SOMETHING_FALSE }} && ${{ env.SOMETHING_TRUE }}", "false && true"},
		{"${{ fromJSON('{}') < 2 }}", "false"},
	}

	updateTestExpressionWorkflow(t, tables, rc)
	for _, table := range tables {
		t.Run("interpolate", func(t *testing.T) {
			assertObject := assert.New(t)
			out := ee.Interpolate(context.Background(), table.in)
			assertObject.Equal(table.out, out, table.in)
		})
	}
}

func updateTestExpressionWorkflow(t *testing.T, tables []struct {
	in  string
	out string
}, rc *RunContext) {
	var envs string
	keys := make([]string, 0, len(rc.Env))
	for k := range rc.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		envs += fmt.Sprintf("  %s: %s\n", k, rc.Env[k])
	}

	// editorconfig-checker-disable
	workflow := fmt.Sprintf(`
name: "Test how expressions are handled on GitHub"
on: push

env:
%s

jobs:
  test-espressions:
    runs-on: ubuntu-latest
    steps:
`, envs)
	// editorconfig-checker-enable
	for _, table := range tables {
		expressionPattern := regexp.MustCompile(`\${{\s*(.+?)\s*}}`)

		expr := expressionPattern.ReplaceAllStringFunc(table.in, func(match string) string {
			return fmt.Sprintf("€{{ %s }}", expressionPattern.ReplaceAllString(match, "$1"))
		})
		name := fmt.Sprintf(`%s -> %s should be equal to %s`, expr, table.in, table.out)
		echo := `run: echo "Done "`
		workflow += fmt.Sprintf("\n      - name: %s\n        %s\n", name, echo)
	}

	file, err := os.Create("../../.github/workflows/test-expressions.yml")
	if err != nil {
		t.Fatal(err)
	}

	_, err = file.WriteString(workflow)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRewriteSubExpression(t *testing.T) {
	table := []struct {
		in  string
		out string
	}{
		{in: "Hello World", out: "Hello World"},
		{in: "${{ true }}", out: "${{ true }}"},
		{in: "${{ true }} ${{ true }}", out: "format('{0} {1}', true, true)"},
		{in: "${{ true || false }} ${{ true && true }}", out: "format('{0} {1}', true || false, true && true)"},
		{in: "${{ '}}' }}", out: "${{ '}}' }}"},
		{in: "${{ '''}}''' }}", out: "${{ '''}}''' }}"},
		{in: "${{ '''' }}", out: "${{ '''' }}"},
		{in: `${{ fromJSON('"}}"') }}`, out: `${{ fromJSON('"}}"') }}`},
		{in: `${{ fromJSON('"\"}}\""') }}`, out: `${{ fromJSON('"\"}}\""') }}`},
		{in: `${{ fromJSON('"''}}"') }}`, out: `${{ fromJSON('"''}}"') }}`},
		{in: "Hello ${{ 'World' }}", out: "format('Hello {0}', 'World')"},
	}

	for _, table := range table {
		t.Run("TestRewriteSubExpression", func(t *testing.T) {
			assertObject := assert.New(t)
			out, err := rewriteSubExpression(context.Background(), table.in, false)
			if err != nil {
				t.Fatal(err)
			}
			assertObject.Equal(table.out, out, table.in)
		})
	}
}

func TestRewriteSubExpressionForceFormat(t *testing.T) {
	table := []struct {
		in  string
		out string
	}{
		{in: "Hello World", out: "Hello World"},
		{in: "${{ true }}", out: "format('{0}', true)"},
		{in: "${{ '}}' }}", out: "format('{0}', '}}')"},
		{in: `${{ fromJSON('"}}"') }}`, out: `format('{0}', fromJSON('"}}"'))`},
		{in: "Hello ${{ 'World' }}", out: "format('Hello {0}', 'World')"},
	}

	for _, table := range table {
		t.Run("TestRewriteSubExpressionForceFormat", func(t *testing.T) {
			assertObject := assert.New(t)
			out, err := rewriteSubExpression(context.Background(), table.in, true)
			if err != nil {
				t.Fatal(err)
			}
			assertObject.Equal(table.out, out, table.in)
		})
	}
}
