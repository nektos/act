package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"github.com/nektos/act/pkg/common/logger"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner/config"
)

func TestSetEnv(t *testing.T) {
	a := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	handler := rc.commandHandler(ctx)

	handler("::set-env name=x::valz\n")
	a.Equal("valz", rc.Env["x"])
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
	handler := rc.commandHandler(ctx)

	handler("::add-path::/zoo\n")
	a.Equal("/zoo", rc.ExtraPath[0])

	handler("::add-path::/boo\n")
	a.Equal("/boo", rc.ExtraPath[1])
}

func TestStopCommands(t *testing.T) {
	_logger, hook := test.NewNullLogger()

	a := assert.New(t)
	ctx := logger.WithLogger(context.Background(), _logger)
	rc := new(RunContext)
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

	a.Contains(messages, "  ::set-env name=x::abcd\n")
}

func TestAddpathADO(t *testing.T) {
	a := assert.New(t)
	ctx := context.Background()
	rc := new(RunContext)
	handler := rc.commandHandler(ctx)

	handler("##[add-path]/zoo\n")
	a.Equal("/zoo", rc.ExtraPath[0])

	handler("##[add-path]/boo\n")
	a.Equal("/boo", rc.ExtraPath[1])
}

func TestAddmask(t *testing.T) {
	_logger, hook := test.NewNullLogger()

	a := assert.New(t)
	ctx := context.Background()
	loggerCtx := logger.WithLogger(ctx, _logger)

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

	cfg := &config.Config{
		Secrets:         map[string]string{},
		InsecureSecrets: false,
	}

	re := captureOutput(t, func() {
		ctx := context.Background()
		ctx = logger.WithJobLogger(ctx, "testjob", cfg, &rc.Masks)

		handler := rc.commandHandler(ctx)
		handler("::add-mask::secret\n")
		handler("::set-output:: token=secret\n")
	})

	a.Equal("[testjob]   \U00002699  ***\n[testjob]   \U00002699  ::set-output:: = token=***\n", re)
}
