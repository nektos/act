package common

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/mattn/go-isatty"
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
func WithJobLogger(ctx context.Context, jobName string, secrets map[string]string, insecureSecrets bool) context.Context {
	mux.Lock()
	defer mux.Unlock()
	formatter := new(stepLogFormatter)
	formatter.color = colors[nextColor%len(colors)]
	formatter.secrets = secrets
	formatter.insecureSecrets = insecureSecrets
	nextColor++

	logger := log.New()
	if TestContext(ctx) {
		fieldLogger := Logger(ctx)
		if fieldLogger != nil {
			logger = fieldLogger.(*log.Logger)
		}
	}
	logger.SetFormatter(formatter)
	logger.SetOutput(ColorableStdout(os.Stdout))
	logger.SetLevel(log.GetLevel())
	rtn := logger.WithFields(log.Fields{"job": jobName, "dryrun": Dryrun(ctx)})

	return WithLogger(ctx, rtn)
}

type stepLogFormatter struct {
	color           int
	secrets         map[string]string
	insecureSecrets bool
}

func (f *stepLogFormatter) Format(entry *log.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	// Replace any secrets in the entry if insecure-secrets flag is not used
	if !f.insecureSecrets {
		for _, v := range f.secrets {
			if v != "" {
				entry.Message = strings.ReplaceAll(entry.Message, v, "***")
			}
		}
	}

	if f.isColored(entry) {
		f.printColored(b, entry)
	} else {
		f.print(b, entry)
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *stepLogFormatter) printColored(b *bytes.Buffer, entry *log.Entry) {
	entry.Message = strings.TrimSuffix(entry.Message, "\n")
	jobName := entry.Data["job"]

	if entry.Data["emoji"] != nil && entry.Data["emoji"].(string) != "" {
		entry.Message = fmt.Sprintf("%s  %s", entry.Data["emoji"], entry.Message)
	}

	if entry.Data["raw_output"] == true {
		fmt.Fprintf(b, "\x1b[%dm|\x1b[0m %s", f.color, entry.Message)
	} else if entry.Data["dryrun"] == true {
		fmt.Fprintf(b, "\x1b[1m\x1b[%dm\x1b[7m*DRYRUN*\x1b[0m \x1b[%dm[%s] \x1b[0m%s", gray, f.color, jobName, entry.Message)
	} else {
		fmt.Fprintf(b, "\x1b[%dm[%s] \x1b[0m%s", f.color, jobName, entry.Message)
	}
}

func (f *stepLogFormatter) print(b *bytes.Buffer, entry *log.Entry) {
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

func (f *stepLogFormatter) isColored(entry *log.Entry) bool {
	return CheckIfColorable(entry.Logger.Out)
}

func ColorableStdout(w io.Writer) io.Writer {
	if CheckIfColorable(w) {
		return w
	}
	return os.Stdout
}

func CheckIfColorable(w io.Writer) bool {
	if !CheckIfTerminal(w) {
		return false
	}

	// https://no-color.org/
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}

	// https://bixense.com/clicolors/
	if f, ok := os.LookupEnv("CLICOLOR_FORCE"); ok && f != "0" {
		return true
	}

	if c, ok := os.LookupEnv("CLICOLOR"); ok {
		if c != "0" {
			return true
		} else if c == "0" {
			return false
		}
	}

	if t, ok := os.LookupEnv("TERM"); ok {
		switch t {
		// safeguard against weird terminals
		case "dumb", "unknown":
			return false
		}
	}

	return true
}

func CheckIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return isatty.IsTerminal(v.Fd()) || isatty.IsCygwinTerminal(v.Fd())
	default:
		return false
	}
}
