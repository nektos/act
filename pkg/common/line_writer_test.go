package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLineWriter(t *testing.T) {
	lines := make([]string, 0)
	lineHandler := func(s string) bool {
		lines = append(lines, s)
		return true
	}

	lineWriter := NewLineWriter(lineHandler)

	assert := assert.New(t)
	write := func(s string) {
		n, err := lineWriter.Write([]byte(s))
		assert.NoError(err)
		assert.Equal(len(s), n, s)
	}

	write("hello")
	write(" ")
	write("world!!\nextra")
	write(" line\n and another\nlast")
	write(" line\n")
	write("no newline here...")

	assert.Len(lines, 4)
	assert.Equal("hello world!!\n", lines[0])
	assert.Equal("extra line\n", lines[1])
	assert.Equal(" and another\n", lines[2])
	assert.Equal("last line\n", lines[3])
}
