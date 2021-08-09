package runner

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"testing"

	"github.com/nektos/act/pkg/model"
	assert "github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v3"
)

func TestEvaluate(t *testing.T) {
	var yml yaml.Node
	err := yml.Encode(map[string][]interface{}{
		"os":  {"Linux", "Windows"},
		"foo": {"bar", "baz"},
	})
	assert.NoError(t, err)

	rc := &RunContext{
		Config: &Config{
			Workdir: ".",
			Secrets: map[string]string{
				"CASE_INSENSITIVE_SECRET": "value",
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
		StepResults: map[string]*stepResult{
			"idwithnothing": {
				Outputs: map[string]string{
					"foowithnothing": "barwithnothing",
				},
				Success: true,
			},
			"id-with-hyphens": {
				Outputs: map[string]string{
					"foo-with-hyphens": "bar-with-hyphens",
				},
				Success: true,
			},
			"id_with_underscores": {
				Outputs: map[string]string{
					"foo_with_underscores": "bar_with_underscores",
				},
				Success: true,
			},
		},
	}
	ee := rc.NewExpressionEvaluator()

	tables := []struct {
		in      string
		out     string
		errMesg string
	}{
		{" 1 ", "1", ""},
		{"1 + 3", "4", ""},
		{"(1 + 3) * -2", "-8", ""},
		{"'my text'", "my text", ""},
		{"contains('my text', 'te')", "true", ""},
		{"contains('my TEXT', 'te')", "true", ""},
		{"contains(['my text'], 'te')", "false", ""},
		{"contains(['foo','bar'], 'bar')", "true", ""},
		{"startsWith('hello world', 'He')", "true", ""},
		{"endsWith('hello world', 'ld')", "true", ""},
		{"format('0:{0} 2:{2} 1:{1}', 'zero', 'one', 'two')", "0:zero 2:two 1:one", ""},
		{"join(['hello'],'octocat')", "hello octocat", ""},
		{"join(['hello','mona','the'],'octocat')", "hello mona the octocat", ""},
		{"join('hello','mona')", "hello mona", ""},
		{"toJSON({'foo':'bar'})", "{\n  \"foo\": \"bar\"\n}", ""},
		{"toJson({'foo':'bar'})", "{\n  \"foo\": \"bar\"\n}", ""},
		{"(fromJSON('{\"foo\":\"bar\"}')).foo", "bar", ""},
		{"(fromJson('{\"foo\":\"bar\"}')).foo", "bar", ""},
		{"hashFiles('**/non-extant-files')", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", ""},
		{"hashFiles('**/non-extant-files', '**/more-non-extant-files')", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", ""},
		{"hashFiles('**/non.extant.files')", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", ""},
		{"hashFiles('**/non''extant''files')", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", ""},
		{"success()", "true", ""},
		{"failure()", "false", ""},
		{"always()", "true", ""},
		{"cancelled()", "false", ""},
		{"github.workflow", "test-workflow", ""},
		{"github.actor", "nektos/act", ""},
		{"github.run_id", "1", ""},
		{"github.run_number", "1", ""},
		{"job.status", "success", ""},
		{"steps.idwithnothing.outputs.foowithnothing", "barwithnothing", ""},
		{"steps.id-with-hyphens.outputs.foo-with-hyphens", "bar-with-hyphens", ""},
		{"steps.id_with_underscores.outputs.foo_with_underscores", "bar_with_underscores", ""},
		{"runner.os", "Linux", ""},
		{"matrix.os", "Linux", ""},
		{"matrix.foo", "bar", ""},
		{"env.key", "value", ""},
		{"secrets.CASE_INSENSITIVE_SECRET", "value", ""},
		{"secrets.case_insensitive_secret", "value", ""},
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
		table := table
		t.Run(table.in, func(t *testing.T) {
			assertObject := assert.New(t)
			out, _, err := ee.Evaluate(table.in)
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
	ee := rc.NewExpressionEvaluator()
	tables := []struct {
		in  string
		out string
	}{
		{" ${{1}} to ${{2}} ", " 1 to 2 "},
		{" ${{ env.KEYWITHNOTHING }} ", " valuewithnothing "},
		{" ${{ env.KEY-WITH-HYPHENS }} ", " value-with-hyphens "},
		{" ${{ env.KEY_WITH_UNDERSCORES }} ", " value_with_underscores "},
		{"${{ secrets.CASE_INSENSITIVE_SECRET }}", "value"},
		{"${{ secrets.case_insensitive_secret }}", "value"},
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
	}

	updateTestExpressionWorkflow(t, tables, rc)
	for _, table := range tables {
		table := table
		t.Run(table.in, func(t *testing.T) {
			assertObject := assert.New(t)
			out := ee.Interpolate(table.in)
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
	for _, table := range tables {
		expressionPattern = regexp.MustCompile(`\${{\s*(.+?)\s*}}`)

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

func TestRewrite(t *testing.T) {
	rc := &RunContext{
		Config: &Config{},
		Run: &model.Run{
			JobID: "job1",
			Workflow: &model.Workflow{
				Jobs: map[string]*model.Job{
					"job1": {},
				},
			},
		},
	}
	ee := rc.NewExpressionEvaluator()

	tables := []struct {
		in string
		re string
	}{
		{"ecole", "ecole"},
		{"ecole.centrale", "ecole['centrale']"},
		{"ecole['centrale']", "ecole['centrale']"},
		{"ecole.centrale.paris", "ecole['centrale']['paris']"},
		{"ecole['centrale'].paris", "ecole['centrale']['paris']"},
		{"ecole.centrale['paris']", "ecole['centrale']['paris']"},
		{"ecole['centrale']['paris']", "ecole['centrale']['paris']"},
		{"ecole.centrale-paris", "ecole['centrale-paris']"},
		{"ecole['centrale-paris']", "ecole['centrale-paris']"},
		{"ecole.centrale_paris", "ecole['centrale_paris']"},
		{"ecole['centrale_paris']", "ecole['centrale_paris']"},
	}

	for _, table := range tables {
		table := table
		t.Run(table.in, func(t *testing.T) {
			assertObject := assert.New(t)
			re := ee.Rewrite(table.in)
			assertObject.Equal(table.re, re, table.in)
		})
	}
}
