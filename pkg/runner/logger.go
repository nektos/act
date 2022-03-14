package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/nektos/act/pkg/common"

	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

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
func WithJobLogger(ctx context.Context, jobName string, config *Config, masks *[]string) context.Context {
	mux.Lock()
	defer mux.Unlock()

	var formatter logrus.Formatter
	if config.JSONLogger {
		formatter = &jobLogJSONFormatter{
			formatter: &logrus.JSONFormatter{},
			masker:    valueMasker(config.InsecureSecrets, config.Secrets, masks),
		}
	} else {
		formatter = &jobLogFormatter{
			color:  colors[nextColor%len(colors)],
			masker: valueMasker(config.InsecureSecrets, config.Secrets, masks),
		}
	}

	nextColor++

	logger := logrus.New()
	logger.SetFormatter(formatter)
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.GetLevel())
	rtn := logger.WithFields(logrus.Fields{"job": jobName, "dryrun": common.Dryrun(ctx)})

	return common.WithLogger(ctx, rtn)
}

func withStepLogger(ctx context.Context, stepName string) context.Context {
	rtn := common.Logger(ctx).WithFields(logrus.Fields{"step": stepName})
	return common.WithLogger(ctx, rtn)
}

type entryProcessor func(entry *logrus.Entry) *logrus.Entry

func valueMasker(insecureSecrets bool, secrets map[string]string, masks *[]string) entryProcessor {
	return func(entry *logrus.Entry) *logrus.Entry {
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

func (f *jobLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
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

func (f *jobLogFormatter) printColored(b *bytes.Buffer, entry *logrus.Entry) {
	entry.Message = strings.TrimSuffix(entry.Message, "\n")
	jobName := entry.Data["job"]

	if entry.Data["raw_output"] == true {
		fmt.Fprintf(b, "\x1b[%dm|\x1b[0m %s", f.color, entry.Message)
	} else if entry.Data["dryrun"] == true {
		fmt.Fprintf(b, "\x1b[1m\x1b[%dm\x1b[7m*DRYRUN*\x1b[0m \x1b[%dm[%s] \x1b[0m%s", gray, f.color, jobName, entry.Message)
	} else {
		fmt.Fprintf(b, "\x1b[%dm[%s] \x1b[0m%s", f.color, jobName, entry.Message)
	}
}

func (f *jobLogFormatter) print(b *bytes.Buffer, entry *logrus.Entry) {
	entry.Message = strings.TrimSuffix(entry.Message, "\n")
	jobName := entry.Data["job"]

	if entry.Data["raw_output"] == true {
		fmt.Fprintf(b, "[%s]   | %s", jobName, entry.Message)
	} else if entry.Data["dryrun"] == true {
		fmt.Fprintf(b, "*DRYRUN* [%s] %s", jobName, entry.Message)
	} else {
		fmt.Fprintf(b, "[%s] %s", jobName, entry.Message)
	}
}

func (f *jobLogFormatter) isColored(entry *logrus.Entry) bool {
	isColored := checkIfTerminal(entry.Logger.Out)

	if force, ok := os.LookupEnv("CLICOLOR_FORCE"); ok && force != "0" {
		isColored = true
	} else if ok && force == "0" {
		isColored = false
	} else if os.Getenv("CLICOLOR") == "0" {
		isColored = false
	}

	return isColored
}

func checkIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

type jobLogJSONFormatter struct {
	masker    entryProcessor
	formatter *logrus.JSONFormatter
}

func (f *jobLogJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return f.formatter.Format(f.masker(entry))
}
