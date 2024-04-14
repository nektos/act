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

type masksContextKey string

const masksContextKeyVal = masksContextKey("logrus.FieldLogger")

// Logger returns the appropriate logger for current context
func Masks(ctx context.Context) *[]string {
	val := ctx.Value(masksContextKeyVal)
	if val != nil {
		if masks, ok := val.(*[]string); ok {
			return masks
		}
	}
	return &[]string{}
}

// WithMasks adds a value to the context for the logger
func WithMasks(ctx context.Context, masks *[]string) context.Context {
	return context.WithValue(ctx, masksContextKeyVal, masks)
}

type JobLoggerFactory interface {
	WithJobLogger() *logrus.Logger
}

type jobLoggerFactoryContextKey string

var jobLoggerFactoryContextKeyVal = (jobLoggerFactoryContextKey)("jobloggerkey")

func WithJobLoggerFactory(ctx context.Context, factory JobLoggerFactory) context.Context {
	return context.WithValue(ctx, jobLoggerFactoryContextKeyVal, factory)
}

// WithJobLogger attaches a new logger to context that is aware of steps
func WithJobLogger(ctx context.Context, jobID string, jobName string, config *Config, masks *[]string, matrix map[string]interface{}) context.Context {
	ctx = WithMasks(ctx, masks)

	var logger *logrus.Logger
	if jobLoggerFactory, ok := ctx.Value(jobLoggerFactoryContextKeyVal).(JobLoggerFactory); ok && jobLoggerFactory != nil {
		logger = jobLoggerFactory.WithJobLogger()
	} else {
		var formatter logrus.Formatter
		if config.JSONLogger {
			formatter = &logrus.JSONFormatter{}
		} else {
			mux.Lock()
			defer mux.Unlock()
			nextColor++
			formatter = &jobLogFormatter{
				color:          colors[nextColor%len(colors)],
				logPrefixJobID: config.LogPrefixJobID,
			}
		}

		logger = logrus.New()
		logger.SetOutput(os.Stdout)
		logger.SetLevel(logrus.GetLevel())
		logger.SetFormatter(formatter)
	}

	logger.SetFormatter(&maskedFormatter{
		Formatter: logger.Formatter,
		masker:    valueMasker(config.InsecureSecrets, config.Secrets),
	})
	rtn := logger.WithFields(logrus.Fields{
		"job":    jobName,
		"jobID":  jobID,
		"dryrun": common.Dryrun(ctx),
		"matrix": matrix,
	}).WithContext(ctx)

	return common.WithLogger(ctx, rtn)
}

func WithCompositeLogger(ctx context.Context, masks *[]string) context.Context {
	ctx = WithMasks(ctx, masks)
	return common.WithLogger(ctx, common.Logger(ctx).WithFields(logrus.Fields{}).WithContext(ctx))
}

func WithCompositeStepLogger(ctx context.Context, stepID string) context.Context {
	val := common.Logger(ctx)
	stepIDs := make([]string, 0)

	if logger, ok := val.(*logrus.Entry); ok {
		if oldStepIDs, ok := logger.Data["stepID"].([]string); ok {
			stepIDs = append(stepIDs, oldStepIDs...)
		}
	}

	stepIDs = append(stepIDs, stepID)

	return common.WithLogger(ctx, common.Logger(ctx).WithFields(logrus.Fields{
		"stepID": stepIDs,
	}).WithContext(ctx))
}

func withStepLogger(ctx context.Context, stepID string, stepName string, stageName string) context.Context {
	rtn := common.Logger(ctx).WithFields(logrus.Fields{
		"step":   stepName,
		"stepID": []string{stepID},
		"stage":  stageName,
	})
	return common.WithLogger(ctx, rtn)
}

type entryProcessor func(entry *logrus.Entry) *logrus.Entry

func valueMasker(insecureSecrets bool, secrets map[string]string) entryProcessor {
	return func(entry *logrus.Entry) *logrus.Entry {
		if insecureSecrets {
			return entry
		}

		masks := Masks(entry.Context)

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

type maskedFormatter struct {
	logrus.Formatter
	masker entryProcessor
}

func (f *maskedFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return f.Formatter.Format(f.masker(entry))
}

type jobLogFormatter struct {
	color          int
	logPrefixJobID bool
}

func (f *jobLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

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

	var job any
	if f.logPrefixJobID {
		job = entry.Data["jobID"]
	} else {
		job = entry.Data["job"]
	}

	debugFlag := ""
	if entry.Level == logrus.DebugLevel {
		debugFlag = "[DEBUG] "
	}

	if entry.Data["raw_output"] == true {
		fmt.Fprintf(b, "\x1b[%dm|\x1b[0m %s", f.color, entry.Message)
	} else if entry.Data["dryrun"] == true {
		fmt.Fprintf(b, "\x1b[1m\x1b[%dm\x1b[7m*DRYRUN*\x1b[0m \x1b[%dm[%s] \x1b[0m%s%s", gray, f.color, job, debugFlag, entry.Message)
	} else {
		fmt.Fprintf(b, "\x1b[%dm[%s] \x1b[0m%s%s", f.color, job, debugFlag, entry.Message)
	}
}

func (f *jobLogFormatter) print(b *bytes.Buffer, entry *logrus.Entry) {
	entry.Message = strings.TrimSuffix(entry.Message, "\n")

	var job any
	if f.logPrefixJobID {
		job = entry.Data["jobID"]
	} else {
		job = entry.Data["job"]
	}

	debugFlag := ""
	if entry.Level == logrus.DebugLevel {
		debugFlag = "[DEBUG] "
	}

	if entry.Data["raw_output"] == true {
		fmt.Fprintf(b, "[%s]   | %s", job, entry.Message)
	} else if entry.Data["dryrun"] == true {
		fmt.Fprintf(b, "*DRYRUN* [%s] %s%s", job, debugFlag, entry.Message)
	} else {
		fmt.Fprintf(b, "[%s] %s%s", job, debugFlag, entry.Message)
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
