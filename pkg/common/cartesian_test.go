package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCartesianProduct(t *testing.T) {
	assert := assert.New(t)
	input := map[string][]interface{}{
		"foo": {1, 2, 3, 4},
		"bar": {"a", "b", "c"},
		"baz": {false, true},
	}

	output := CartesianProduct(input)
	assert.Len(output, 24)

	for _, v := range output {
		assert.Len(v, 3)

		assert.Contains(v, "foo")
		assert.Contains(v, "bar")
		assert.Contains(v, "baz")
	}

  input = map[string][]interface{}{
		"foo": {1, 2, 3, 4},
		"bar": {},
		"baz": {false, true},
  }
  output = CartesianProduct(input)
  assert.Len(output, 0)

  input = map[string][]interface{}{}
  output = CartesianProduct(input)
  assert.Len(output, 0)
}

