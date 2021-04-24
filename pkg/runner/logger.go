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
func WithJobLogger(ctx context.Context, jobName string, secrets map[string]string, insecureSecrets bool) context.Context {
	mux.Lock()
	defer mux.Unlock()
	formatter := new(stepLogFormatter)
	formatter.color = colors[nextColor%len(colors)]
	formatter.secrets = secrets
	formatter.insecureSecrets = insecureSecrets
	nextColor++

	logger := logrus.New()
	logger.SetFormatter(formatter)
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.GetLevel())
	rtn := logger.WithFields(logrus.Fields{"job": jobName, "dryrun": common.Dryrun(ctx)})

	return common.WithLogger(ctx, rtn)
}

type stepLogFormatter struct {
	color           int
	secrets         map[string]string
	insecureSecrets bool
}

func (f *stepLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	// Replace any secrets in the entry if insecure-secrets flag is not used
	if !f.insecureSecrets {
		for _, v := range f.secrets {
			entry.Message = strings.ReplaceAll(entry.Message, v, "***")
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

func (f *stepLogFormatter) printColored(b *bytes.Buffer, entry *logrus.Entry) {
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

func (f *stepLogFormatter) print(b *bytes.Buffer, entry *logrus.Entry) {
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

func (f *stepLogFormatter) isColored(entry *logrus.Entry) bool {
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
