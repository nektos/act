package runner

import (
	"testing"

	"github.com/nektos/act/pkg/model"
	a "github.com/stretchr/testify/assert"
)

func TestEvaluate(t *testing.T) {
	rc := &RunContext{
		Config: &Config{
			Workdir: ".",
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
	}

	for _, table := range tables {
		table := table
		t.Run(table.in, func(t *testing.T) {
			assert := a.New(t)
			out, err := ee.Evaluate(table.in)
			if table.errMesg == "" {
				assert.NoError(err, table.in)
				assert.Equal(table.out, out, table.in)
			} else {
				assert.Error(err, table.in)
				assert.Equal(table.errMesg, err.Error(), table.in)
			}
		})
	}
}

func TestInterpolate(t *testing.T) {
	rc := &RunContext{
		Config: &Config{
			Workdir: ".",
		},
		Env: map[string]string{
			"KEYWITHNOTHING":       "valuewithnothing",
			"KEY-WITH-HYPHENS":     "value-with-hyphens",
			"KEY_WITH_UNDERSCORES": "value_with_underscores",
			"TRUE":                 "true",
			"FALSE":                "false",
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
		{"${{ env.UNKNOWN }}", ""},
		{"${{ env.TRUE }}", "true"},
		{"${{ env.FALSE }}", "false"},
		{"${{ !env.TRUE }}", "!true"},
		{"${{ !env.FALSE }}", "!false"},
	}

	for _, table := range tables {
		table := table
		t.Run(table.in, func(t *testing.T) {
			assert := a.New(t)
			out := ee.Interpolate(table.in)
			assert.Equal(table.out, out, table.in)
		})
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
			assert := a.New(t)
			re := ee.Rewrite(table.in)
			assert.Equal(table.re, re, table.in)
		})
	}
}
