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
	rc.StepResults = make(map[string]*stepResult)
	handler := rc.commandHandler(ctx)

	rc.CurrentStep = "my-step"
	rc.StepResults[rc.CurrentStep] = &stepResult{
		Outputs: make(map[string]string),
	}
	handler("::set-output name=x::valz\n")
	assert.Equal("valz", rc.StepResults["my-step"].Outputs["x"])
}

func TestAddpath(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	handler := rc.commandHandler(ctx)

	handler("::add-path::/zoo\n")
	assert.Equal("/zoo", rc.ExtraPath[0])

	handler("::add-path::/boo\n")
	assert.Equal("/boo", rc.ExtraPath[1])
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
