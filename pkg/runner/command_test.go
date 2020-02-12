package runner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetEnv(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	handler := rc.commandHandler(ctx)

	handler("::set-env name=x::valz\n")
	assert.Equal("valz", rc.Env["x"])
}

func TestSetOutput(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	handler := rc.commandHandler(ctx)

	handler("::set-output name=x::valz\n")
	assert.Equal("valz", rc.Outputs["x"])
}

func TestAddpath(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	handler := rc.commandHandler(ctx)

	handler("::add-path::/zoo\n")
	assert.Equal("/zoo:", rc.Env["PATH"])

	handler("::add-path::/booo\n")
	assert.Equal("/booo:/zoo:", rc.Env["PATH"])
}

func TestStopCommands(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	handler := rc.commandHandler(ctx)

	handler("::set-env name=x::valz\n")
	assert.Equal("valz", rc.Env["x"])
	handler("::stop-commands::my-end-token\n")
	handler("::set-env name=x::abcd\n")
	assert.Equal("valz", rc.Env["x"])
	handler("::my-end-token::\n")
	handler("::set-env name=x::abcd\n")
	assert.Equal("abcd", rc.Env["x"])
}
