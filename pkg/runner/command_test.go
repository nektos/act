package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

func TestSetEnv(t *testing.T) {
	a := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	rc.Env = map[string]string{"ACTIONS_ALLOW_UNSECURE_COMMANDS": "true"}
	handler := rc.commandHandler(ctx)

	handler("::set-env name=x::valz\n")
	a.Equal("valz", rc.Env["x"])
}

func TestSetEnvBlocked(t *testing.T) {
	a := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	handler := rc.commandHandler(ctx)

	handler("::set-env name=x::valz\n")
	a.Equal("", rc.Env["x"])
}

func TestSetOutput(t *testing.T) {
	a := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	rc.StepResults = make(map[string]*model.StepResult)
	handler := rc.commandHandler(ctx)

	rc.CurrentStep = "my-step"
	rc.StepResults[rc.CurrentStep] = &model.StepResult{
		Outputs: make(map[string]string),
	}
	handler("::set-output name=x::valz\n")
	a.Equal("valz", rc.StepResults["my-step"].Outputs["x"])

	handler("::set-output name=x::percent2%25\n")
	a.Equal("percent2%", rc.StepResults["my-step"].Outputs["x"])

	handler("::set-output name=x::percent2%25%0Atest\n")
	a.Equal("percent2%\ntest", rc.StepResults["my-step"].Outputs["x"])

	handler("::set-output name=x::percent2%25%0Atest another3%25test\n")
	a.Equal("percent2%\ntest another3%test", rc.StepResults["my-step"].Outputs["x"])

	handler("::set-output name=x%3A::percent2%25%0Atest\n")
	a.Equal("percent2%\ntest", rc.StepResults["my-step"].Outputs["x:"])

	handler("::set-output name=x%3A%2C%0A%25%0D%3A::percent2%25%0Atest\n")
	a.Equal("percent2%\ntest", rc.StepResults["my-step"].Outputs["x:,\n%\r:"])
}

func TestAddpath(t *testing.T) {
	a := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	rc.Env = map[string]string{"ACTIONS_ALLOW_UNSECURE_COMMANDS": "true"}
	handler := rc.commandHandler(ctx)

	handler("::add-path::/zoo\n")
	a.Equal("/zoo", rc.ExtraPath[0])

	handler("::add-path::/boo\n")
	a.Equal("/boo", rc.ExtraPath[0])
}

func TestAddPathBlocked(t *testing.T) {
	a := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	handler := rc.commandHandler(ctx)

	handler("::add-path::/zoo\n")
	a.Empty(rc.ExtraPath)
}

func TestStopCommands(t *testing.T) {
	logger, hook := test.NewNullLogger()

	a := assert.New(t)
	ctx := common.WithLogger(context.Background(), logger)
	rc := new(RunContext)
	rc.Env = map[string]string{"ACTIONS_ALLOW_UNSECURE_COMMANDS": "true"}
	handler := rc.commandHandler(ctx)

	handler("::set-env name=x::valz\n")
	a.Equal("valz", rc.Env["x"])
	handler("::stop-commands::my-end-token\n")
	handler("::set-env name=x::abcd\n")
	a.Equal("valz", rc.Env["x"])
	handler("::my-end-token::\n")
	handler("::set-env name=x::abcd\n")
	a.Equal("abcd", rc.Env["x"])

	messages := make([]string, 0)
	for _, entry := range hook.AllEntries() {
		messages = append(messages, entry.Message)
	}

	a.Contains(messages, "  \U00002699  ::set-env name=x::abcd\n")
}

func TestAddpathADO(t *testing.T) {
	a := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	rc.Env = map[string]string{"ACTIONS_ALLOW_UNSECURE_COMMANDS": "true"}
	handler := rc.commandHandler(ctx)

	handler("##[add-path]/zoo\n")
	a.Equal("/zoo", rc.ExtraPath[0])

	handler("##[add-path]/boo\n")
	a.Equal("/boo", rc.ExtraPath[0])
}

func TestAddmask(t *testing.T) {
	logger, hook := test.NewNullLogger()

	a := assert.New(t)
	ctx := context.Background()
	loggerCtx := common.WithLogger(ctx, logger)

	rc := new(RunContext)
	handler := rc.commandHandler(loggerCtx)
	handler("::add-mask::my-secret-value\n")

	a.Equal("  \U00002699  ***", hook.LastEntry().Message)
	a.NotEqual("  \U00002699  *my-secret-value", hook.LastEntry().Message)
}

// based on https://stackoverflow.com/a/10476304
func captureOutput(t *testing.T, f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	outC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		if err != nil {
			a := assert.New(t)
			a.Fail("io.Copy failed")
		}
		outC <- buf.String()
	}()

	w.Close()
	os.Stdout = old
	out := <-outC

	return out
}

func TestAddmaskUsemask(t *testing.T) {
	rc := new(RunContext)
	rc.StepResults = make(map[string]*model.StepResult)
	rc.CurrentStep = "my-step"
	rc.StepResults[rc.CurrentStep] = &model.StepResult{
		Outputs: make(map[string]string),
	}

	a := assert.New(t)

	config := &Config{
		Secrets:         map[string]string{},
		InsecureSecrets: false,
	}

	re := captureOutput(t, func() {
		ctx := context.Background()
		ctx = WithJobLogger(ctx, "0", "testjob", config, &rc.Masks, map[string]interface{}{})

		handler := rc.commandHandler(ctx)
		handler("::add-mask::secret\n")
		handler("::set-output:: token=secret\n")
	})

	a.Equal("[testjob]   \U00002699  ***\n[testjob]   \U00002699  ::set-output:: = token=***\n", re)
}

func TestSaveState(t *testing.T) {
	rc := &RunContext{
		CurrentStep: "step",
		StepResults: map[string]*model.StepResult{},
	}

	ctx := context.Background()

	handler := rc.commandHandler(ctx)
	handler("::save-state name=state-name::state-value\n")

	assert.Equal(t, "state-value", rc.IntraActionState["step"]["state-name"])
}

func TestDebugCommandHiddenByDefault(t *testing.T) {
	rc := new(RunContext)
	a := assert.New(t)

	out := captureOutput(t, func() {
		ctx := WithJobLogger(context.Background(), "0", "testjob", &Config{}, &rc.Masks, map[string]interface{}{})
		handler := rc.commandHandler(ctx)
		handler("::debug::noisy details\n")
	})

	a.Equal("", out)
}

func TestDebugCommandShownWithActionsStepDebug(t *testing.T) {
	tables := []struct {
		name   string
		env    map[string]string
		config *Config
	}{
		{"secret", nil, &Config{Secrets: map[string]string{"ACTIONS_STEP_DEBUG": "true"}}},
		{"var", nil, &Config{Vars: map[string]string{"ACTIONS_STEP_DEBUG": "TRUE"}}},
		{"config env", nil, &Config{Env: map[string]string{"ACTIONS_STEP_DEBUG": "true"}}},
		{"job env", map[string]string{"ACTIONS_STEP_DEBUG": "true"}, &Config{}},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			rc := &RunContext{Config: table.config, Env: table.env}
			a := assert.New(t)

			out := captureOutput(t, func() {
				ctx := WithJobLogger(context.Background(), "0", "testjob", table.config, &rc.Masks, map[string]interface{}{})
				handler := rc.commandHandler(ctx)
				handler("::debug::useful details\n")
			})

			a.Equal("[testjob]   \U0001F4AC  ::debug::useful details\n", out)
		})
	}
}

func TestDebugCommandNotShownWhenStepDebugFalse(t *testing.T) {
	rc := &RunContext{Config: &Config{Secrets: map[string]string{"ACTIONS_STEP_DEBUG": "false"}}}
	a := assert.New(t)

	out := captureOutput(t, func() {
		ctx := WithJobLogger(context.Background(), "0", "testjob", rc.Config, &rc.Masks, map[string]interface{}{})
		handler := rc.commandHandler(ctx)
		handler("::debug::noisy details\n")
	})

	a.Equal("", out)
}

func TestDebugCommandJSONLoggerMasksSecrets(t *testing.T) {
	config := &Config{
		JSONLogger: true,
		Secrets:    map[string]string{"ACTIONS_STEP_DEBUG": "true", "TOKEN": "s3cr3t-value"},
	}
	rc := &RunContext{Config: config}
	a := assert.New(t)

	out := captureOutput(t, func() {
		ctx := WithJobLogger(context.Background(), "0", "testjob", config, &rc.Masks, map[string]interface{}{})
		handler := rc.commandHandler(ctx)
		handler("::debug::token is s3cr3t-value\n")
	})

	a.Contains(out, "token is ***")
	a.NotContains(out, "s3cr3t-value")
}

func TestDebugCommandMasksEscapedSecrets(t *testing.T) {
	config := &Config{
		Secrets: map[string]string{"ACTIONS_STEP_DEBUG": "true", "TOKEN": "s3cr%et"},
	}
	rc := &RunContext{Config: config}
	a := assert.New(t)

	out := captureOutput(t, func() {
		ctx := WithJobLogger(context.Background(), "0", "testjob", config, &rc.Masks, map[string]interface{}{})
		handler := rc.commandHandler(ctx)
		// A command's data is encoded, so the secret arrives as s3cr%25et.
		handler("::debug::token is s3cr%25et\n")
		handler("::add-mask::multi%0Aline\n")
		handler("::debug::value is multi%0Aline\n")
	})

	a.Contains(out, "token is ***")
	a.Contains(out, "value is ***")
	a.NotContains(out, "s3cr%25et")
	a.NotContains(out, "s3cr%et")
	a.NotContains(out, "multi%0Aline")
}

func TestDebugCommandShownInCompositeWithSecretStepDebug(t *testing.T) {
	parent := &RunContext{Config: &Config{Secrets: map[string]string{"ACTIONS_STEP_DEBUG": "true"}}}
	// composite run contexts get a copy of the config without secrets
	rc := &RunContext{Config: &Config{}, Parent: parent}
	a := assert.New(t)

	out := captureOutput(t, func() {
		ctx := WithJobLogger(context.Background(), "0", "testjob", rc.Config, &rc.Masks, map[string]interface{}{})
		handler := rc.commandHandler(ctx)
		handler("::debug::from composite\n")
	})

	a.Equal("[testjob]   \U0001F4AC  ::debug::from composite\n", out)
}
