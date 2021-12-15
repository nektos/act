package logger

import (
	"bytes"
	"context"
	"fmt"
	"github.com/nektos/act/pkg/common/utils"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/nektos/act/pkg/common/dryrun"
	"github.com/nektos/act/pkg/runner/config"

	log "github.com/sirupsen/logrus"
)

type loggerContextKey string

const loggerContextKeyVal = loggerContextKey("log.Ext1FieldLogger")

// Logger returns the appropriate logger for current context
func Logger(ctx context.Context) log.Ext1FieldLogger {
	val := ctx.Value(loggerContextKeyVal)
	if val != nil {
		if logger, ok := val.(log.Ext1FieldLogger); ok {
			return logger
		}
	}
	return log.StandardLogger()
}

// WithLogger adds a value to the context for the logger
func WithLogger(ctx context.Context, logger log.Ext1FieldLogger) context.Context {
	return context.WithValue(ctx, loggerContextKeyVal, logger)
}

const (
	// nocolor = 0
	red     = 31
	green   = 32
	yellow  = 33
	blue    = 34
	magenta = 35
	cyan    = 36
	gray    = 37
)

var colors []int
var nextColor int
var mux sync.Mutex

func init() {
	nextColor = 0
	colors = []int{
		blue, yellow, green, magenta, red, gray, cyan,
	}
}

// WithJobLogger attaches a new logger to context that is aware of steps
func WithJobLogger(ctx context.Context, jobName string, config *config.Config, masks *[]string) context.Context {
	mux.Lock()
	defer mux.Unlock()

	var formatter log.Formatter
	if config.JSONLogger {
		formatter = &jobLogJSONFormatter{
			formatter: &log.JSONFormatter{},
			masker:    valueMasker(config.InsecureSecrets, config.Secrets, masks),
		}
	} else {
		formatter = &jobLogFormatter{
			color:  colors[nextColor%len(colors)],
			masker: valueMasker(config.InsecureSecrets, config.Secrets, masks),
		}
	}

	logger := log.New()
	logger.SetFormatter(formatter)
	logger.SetOutput(os.Stdout)
	logger.SetLevel(log.GetLevel())
	rtn := logger.WithFields(log.Fields{"job": jobName, "dryrun": dryrun.Dryrun(ctx)})

	return WithLogger(ctx, rtn)
}

func WithStepLogger(ctx context.Context, stepName string) context.Context {
	rtn := Logger(ctx).WithFields(log.Fields{"step": stepName})
	return WithLogger(ctx, rtn)
}

type entryProcessor func(entry *log.Entry) *log.Entry

func valueMasker(insecureSecrets bool, secrets map[string]string, masks *[]string) entryProcessor {
	return func(entry *log.Entry) *log.Entry {
		if insecureSecrets {
			return entry
		}

		for _, v := range secrets {
			if v != "" {
				entry.Message = strings.ReplaceAll(entry.Message, v, "***")
			}
		}

		for _, v := range *masks {
			if v != "" {
				entry.Message = strings.ReplaceAll(entry.Message, v, "***")
			}
		}

		return entry
	}
}

type jobLogFormatter struct {
	color  int
	masker entryProcessor
}

func (f *jobLogFormatter) Format(entry *log.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	entry = f.masker(entry)

	if f.isColored(entry) {
		f.printColored(b, entry)
	} else {
		f.print(b, entry)
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *jobLogFormatter) printColored(b *bytes.Buffer, entry *log.Entry) {
	entry.Message = strings.TrimSuffix(entry.Message, "\n")
	jobName := entry.Data["job"]

	if entry.Data["emoji"] != nil && entry.Data["emoji"].(string) != "" {
		entry.Message = fmt.Sprintf("%s  %s", entry.Data["emoji"], entry.Message)
	}

	if entry.Data["raw_output"] == true {
		_, _ = fmt.Fprintf(b, "\x1b[%dm|\x1b[0m %s", f.color, entry.Message)
	} else if entry.Data["dryrun"] == true {
		_, _ = fmt.Fprintf(b, "\x1b[1m\x1b[%dm\x1b[7m*DRYRUN*\x1b[0m \x1b[%dm[%s] \x1b[0m%s", gray, f.color, jobName, entry.Message)
	} else {
		_, _ = fmt.Fprintf(b, "\x1b[%dm[%s] \x1b[0m%s", f.color, jobName, entry.Message)
	}
}

func (f *jobLogFormatter) print(b *bytes.Buffer, entry *log.Entry) {
	entry.Message = strings.TrimSuffix(entry.Message, "\n")
	jobName := entry.Data["job"]

	if entry.Data["raw_output"] == true {
		_, _ = fmt.Fprintf(b, "[%s]   | %s", jobName, entry.Message)
	} else if entry.Data["dryrun"] == true {
		_, _ = fmt.Fprintf(b, "*DRYRUN* [%s] %s", jobName, entry.Message)
	} else {
		_, _ = fmt.Fprintf(b, "[%s] %s", jobName, entry.Message)
	}
}

func (f *jobLogFormatter) isColored(entry *log.Entry) bool {
	return utils.CheckIfColorable(entry.Logger.Out)
}

type jobLogJSONFormatter struct {
	masker    entryProcessor
	formatter *log.JSONFormatter
}

func (f *jobLogJSONFormatter) Format(entry *log.Entry) ([]byte, error) {
	return f.formatter.Format(f.masker(entry))
}

func ColorableStdout(w io.Writer) io.Writer {
	if utils.CheckIfColorable(w) {
		return w
	}
	return os.Stdout
}
