package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
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
	ssecrets := make([]string, 0, len(secrets))
	for _, v := range secrets {
		ssecrets = append(ssecrets, v)
	}
	return func(entry *logrus.Entry) *logrus.Entry {
		if insecureSecrets {
			return entry
		}

		masks := Masks(entry.Context)
		// Dup copies Data shallowly and omits Message, Level, Caller, and Buffer.
		// Mask the message and Data on that copy. Nested maps and slices are
		// deep-copied: matrix is the live job matrix, and kvPairs is reused to
		// set outputs and env, so neither map may be mutated.
		masked := entry.Dup()
		masked.Level = entry.Level
		masked.Caller = entry.Caller
		masked.Buffer = entry.Buffer
		masked.Message = maskValues(entry.Message, ssecrets, *masks)
		if len(entry.Data) == 0 {
			masked.Data = entry.Data
			return masked
		}
		for k, v := range masked.Data {
			masked.Data[k] = maskLogValue(v, ssecrets, *masks)
		}
		return masked
	}
}

// maskLogValue redacts secrets in one structured field. Maps and slices are
// copied so the caller's containers stay unchanged. Bools and numbers are
// returned as-is.
func maskLogValue(v any, secrets, masks []string) any {
	switch val := v.(type) {
	case string:
		if val == "" {
			return val
		}
		return maskValues(val, secrets, masks)
	case []byte:
		if len(val) == 0 {
			return val
		}
		return []byte(maskValues(string(val), secrets, masks))
	case map[string]string:
		return maskStringMap(val, secrets, masks)
	case map[string]any:
		return maskAnyMap(val, secrets, masks)
	case logrus.Fields:
		// Conversion shares the backing map; maskAnyMap only reads it.
		return logrus.Fields(maskAnyMap(map[string]any(val), secrets, masks))
	case []string:
		return maskStringSlice(val, secrets, masks)
	case []any:
		return maskAnySlice(val, secrets, masks)
	default:
		return v
	}
}

func maskStringMap(in map[string]string, secrets, masks []string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if v == "" {
			out[k] = v
			continue
		}
		out[k] = maskValues(v, secrets, masks)
	}
	return out
}

func maskAnyMap(in map[string]any, secrets, masks []string) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = maskLogValue(v, secrets, masks)
	}
	return out
}

func maskStringSlice(in []string, secrets, masks []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, v := range in {
		if v == "" {
			out[i] = v
			continue
		}
		out[i] = maskValues(v, secrets, masks)
	}
	return out
}

func maskAnySlice(in []any, secrets, masks []string) []any {
	if in == nil {
		return nil
	}
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = maskLogValue(v, secrets, masks)
	}
	return out
}

// maskValues redacts secrets and ::add-mask:: values in one pass.
// Each value is also masked in its workflow-command encoded form (`%` as `%25`,
// line breaks as `%0D`/`%0A`), because raw `::debug::` and similar lines are
// logged as written and would otherwise show an encoded secret.
// Longer values are applied first so a short value nested inside a longer one
// cannot be replaced early and leave the rest of the longer value visible.
// Equal lengths break ties lexically so the result does not depend on map iteration.
func maskValues(message string, secrets, masks []string) string {
	values := make([]string, 0, 2*(len(secrets)+len(masks)))
	seen := make(map[string]struct{}, 2*(len(secrets)+len(masks)))
	add := func(v string) {
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		values = append(values, v)
	}
	for _, group := range [][]string{secrets, masks} {
		for _, v := range group {
			add(v)
			add(escapeCommandData(v))
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if len(values[i]) != len(values[j]) {
			return len(values[i]) > len(values[j])
		}
		return values[i] < values[j]
	})
	if len(values) == 0 {
		return message
	}
	oldnew := make([]string, 0, len(values)*2)
	for _, v := range values {
		oldnew = append(oldnew, v, "***")
	}
	return strings.NewReplacer(oldnew...).Replace(message)
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
	entry.Message = sanitizeTerminal(strings.TrimSuffix(entry.Message, "\n"))

	var job any
	if f.logPrefixJobID {
		job = entry.Data["jobID"]
	} else {
		job = entry.Data["job"]
	}
	if name, ok := job.(string); ok {
		job = sanitizeTerminal(name)
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
	entry.Message = sanitizeTerminal(strings.TrimSuffix(entry.Message, "\n"))

	var job any
	if f.logPrefixJobID {
		job = entry.Data["jobID"]
	} else {
		job = entry.Data["job"]
	}
	if name, ok := job.(string); ok {
		job = sanitizeTerminal(name)
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
