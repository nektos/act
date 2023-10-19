package exprparser

import (
	"path/filepath"
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestFunctionContains(t *testing.T) {
	table := []struct {
		input    string
		expected interface{}
		name     string
	}{
		{"contains('search', 'item') }}", false, "contains-str-str"},
		{`cOnTaInS('Hello', 'll') }}`, true, "contains-str-casing"},
		{`contains('HELLO', 'll') }}`, true, "contains-str-casing"},
		{`contains('3.141592', 3.14) }}`, true, "contains-str-number"},
		{`contains(3.141592, '3.14') }}`, true, "contains-number-str"},
		{`contains(3.141592, 3.14) }}`, true, "contains-number-number"},
		{`contains(true, 'u') }}`, true, "contains-bool-str"},
		{`contains(null, '') }}`, true, "contains-null-str"},
		{`contains(fromJSON('["first","second"]'), 'first') }}`, true, "contains-item"},
		{`contains(fromJSON('[null,"second"]'), '') }}`, true, "contains-item-null-empty-str"},
		{`contains(fromJSON('["","second"]'), null) }}`, true, "contains-item-empty-str-null"},
		{`contains(fromJSON('[true,"second"]'), 'true') }}`, false, "contains-item-bool-arr"},
		{`contains(fromJSON('["true","second"]'), true) }}`, false, "contains-item-str-bool"},
		{`contains(fromJSON('[3.14,"second"]'), '3.14') }}`, true, "contains-item-number-str"},
		{`contains(fromJSON('[3.14,"second"]'), 3.14) }}`, true, "contains-item-number-number"},
		{`contains(fromJSON('["","second"]'), fromJSON('[]')) }}`, false, "contains-item-str-arr"},
		{`contains(fromJSON('["","second"]'), fromJSON('{}')) }}`, false, "contains-item-str-obj"},
	}

	env := &EvaluationEnvironment{}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewInterpeter(env, Config{}).Evaluate(tt.input, DefaultStatusCheckNone)
			assert.Nil(t, err)

			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestFunctionStartsWith(t *testing.T) {
	table := []struct {
		input    string
		expected interface{}
		name     string
	}{
		{"startsWith('search', 'se') }}", true, "startswith-string"},
		{"startsWith('search', 'sa') }}", false, "startswith-string"},
		{"startsWith('123search', '123s') }}", true, "startswith-string"},
		{"startsWith(123, 's') }}", false, "startswith-string"},
		{"startsWith(123, '12') }}", true, "startswith-string"},
		{"startsWith('123', 12) }}", true, "startswith-string"},
		{"startsWith(null, '42') }}", false, "startswith-string"},
		{"startsWith('null', null) }}", true, "startswith-string"},
		{"startsWith('null', '') }}", true, "startswith-string"},
	}

	env := &EvaluationEnvironment{}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewInterpeter(env, Config{}).Evaluate(tt.input, DefaultStatusCheckNone)
			assert.Nil(t, err)

			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestFunctionEndsWith(t *testing.T) {
	table := []struct {
		input    string
		expected interface{}
		name     string
	}{
		{"endsWith('search', 'ch') }}", true, "endsWith-string"},
		{"endsWith('search', 'sa') }}", false, "endsWith-string"},
		{"endsWith('search123s', '123s') }}", true, "endsWith-string"},
		{"endsWith(123, 's') }}", false, "endsWith-string"},
		{"endsWith(123, '23') }}", true, "endsWith-string"},
		{"endsWith('123', 23) }}", true, "endsWith-string"},
		{"endsWith(null, '42') }}", false, "endsWith-string"},
		{"endsWith('null', null) }}", true, "endsWith-string"},
		{"endsWith('null', '') }}", true, "endsWith-string"},
	}

	env := &EvaluationEnvironment{}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewInterpeter(env, Config{}).Evaluate(tt.input, DefaultStatusCheckNone)
			assert.Nil(t, err)

			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestFunctionJoin(t *testing.T) {
	table := []struct {
		input    string
		expected interface{}
		name     string
	}{
		{"join(fromJSON('[\"a\", \"b\"]'), ',')", "a,b", "join-arr"},
		{"join('string', ',')", "string", "join-str"},
		{"join(1, ',')", "1", "join-number"},
		{"join(null, ',')", "", "join-number"},
		{"join(fromJSON('[\"a\", \"b\", null]'), null)", "ab", "join-number"},
		{"join(fromJSON('[\"a\", \"b\"]'))", "a,b", "join-number"},
		{"join(fromJSON('[\"a\", \"b\", null]'), 1)", "a1b1", "join-number"},
	}

	env := &EvaluationEnvironment{}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewInterpeter(env, Config{}).Evaluate(tt.input, DefaultStatusCheckNone)
			assert.Nil(t, err)

			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestFunctionToJSON(t *testing.T) {
	table := []struct {
		input    string
		expected interface{}
		name     string
	}{
		{"toJSON(env) }}", "{\n  \"key\": \"value\"\n}", "toJSON"},
		{"toJSON(null)", "null", "toJSON-null"},
	}

	env := &EvaluationEnvironment{
		Env: map[string]string{
			"key": "value",
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewInterpeter(env, Config{}).Evaluate(tt.input, DefaultStatusCheckNone)
			assert.Nil(t, err)

			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestFunctionFromJSON(t *testing.T) {
	table := []struct {
		input    string
		expected interface{}
		name     string
	}{
		{"fromJSON('{\"foo\":\"bar\"}') }}", map[string]interface{}{
			"foo": "bar",
		}, "fromJSON"},
	}

	env := &EvaluationEnvironment{}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewInterpeter(env, Config{}).Evaluate(tt.input, DefaultStatusCheckNone)
			assert.Nil(t, err)

			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestFunctionHashFiles(t *testing.T) {
	table := []struct {
		input    string
		expected interface{}
		name     string
	}{
		{"hashFiles('**/non-extant-files') }}", "", "hash-non-existing-file"},
		{"hashFiles('**/non-extant-files', '**/more-non-extant-files') }}", "", "hash-multiple-non-existing-files"},
		{"hashFiles('./for-hashing-1.txt') }}", "66a045b452102c59d840ec097d59d9467e13a3f34f6494e539ffd32c1bb35f18", "hash-single-file"},
		{"hashFiles('./for-hashing-*.txt') }}", "8e5935e7e13368cd9688fe8f48a0955293676a021562582c7e848dafe13fb046", "hash-multiple-files"},
		{"hashFiles('./for-hashing-*.txt', '!./for-hashing-2.txt') }}", "66a045b452102c59d840ec097d59d9467e13a3f34f6494e539ffd32c1bb35f18", "hash-negative-pattern"},
		{"hashFiles('./for-hashing-**') }}", "c418ba693753c84115ced0da77f876cddc662b9054f4b129b90f822597ee2f94", "hash-multiple-files-and-directories"},
		{"hashFiles('./for-hashing-3/**') }}", "6f5696b546a7a9d6d42a449dc9a56bef244aaa826601ef27466168846139d2c2", "hash-nested-directories"},
		{"hashFiles('./for-hashing-3/**/nested-data.txt') }}", "8ecadfb49f7f978d0a9f3a957e9c8da6cc9ab871f5203b5d9f9d1dc87d8af18c", "hash-nested-directories-2"},
	}

	env := &EvaluationEnvironment{}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			workdir, err := filepath.Abs("testdata")
			assert.Nil(t, err)
			output, err := NewInterpeter(env, Config{WorkingDir: workdir}).Evaluate(tt.input, DefaultStatusCheckNone)
			assert.Nil(t, err)

			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestFunctionFormat(t *testing.T) {
	table := []struct {
		input    string
		expected interface{}
		error    interface{}
		name     string
	}{
		{"format('text')", "text", nil, "format-plain-string"},
		{"format('Hello {0} {1} {2}!', 'Mona', 'the', 'Octocat')", "Hello Mona the Octocat!", nil, "format-with-placeholders"},
		{"format('{{Hello {0} {1} {2}!}}', 'Mona', 'the', 'Octocat')", "{Hello Mona the Octocat!}", nil, "format-with-escaped-braces"},
		{"format('{{0}}', 'test')", "{0}", nil, "format-with-escaped-braces"},
		{"format('{{{0}}}', 'test')", "{test}", nil, "format-with-escaped-braces-and-value"},
		{"format('}}')", "}", nil, "format-output-closing-brace"},
		{`format('Hello "{0}" {1} {2} {3} {4}', null, true, -3.14, NaN, Infinity)`, `Hello "" true -3.14 NaN Infinity`, nil, "format-with-primitives"},
		{`format('Hello "{0}" {1} {2}', fromJSON('[0, true, "abc"]'), fromJSON('[{"a":1}]'), fromJSON('{"a":{"b":1}}'))`, `Hello "Array" Array Object`, nil, "format-with-complex-types"},
		{"format(true)", "true", nil, "format-with-primitive-args"},
		{"format('echo Hello {0} ${{Test}}', github.undefined_property)", "echo Hello  ${Test}", nil, "format-with-undefined-value"},
		{"format('{0}}', '{1}', 'World')", nil, "Closing bracket without opening one. The following format string is invalid: '{0}}'", "format-invalid-format-string"},
		{"format('{0', '{1}', 'World')", nil, "Unclosed brackets. The following format string is invalid: '{0'", "format-invalid-format-string"},
		{"format('{2}', '{1}', 'World')", "", "The following format string references more arguments than were supplied: '{2}'", "format-invalid-replacement-reference"},
		{"format('{2147483648}')", "", "The following format string is invalid: '{2147483648}'", "format-invalid-replacement-reference"},
		{"format('{0} {1} {2} {3}', 1.0, 1.1, 1234567890.0, 12345678901234567890.0)", "1 1.1 1234567890 1.23456789012346E+19", nil, "format-floats"},
	}

	env := &EvaluationEnvironment{
		Github: &model.GithubContext{},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			output, err := NewInterpeter(env, Config{}).Evaluate(tt.input, DefaultStatusCheckNone)
			if tt.error != nil {
				assert.Equal(t, tt.error, err.Error())
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.expected, output)
			}
		})
	}
}
