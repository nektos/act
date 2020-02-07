package runner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	//nocolor = 0
	red    = 31
	green  = 32
	yellow = 33
	blue   = 36
	gray   = 37
)

// NewJobLogger gets the logger for the Job
func NewJobLogger(jobName string, dryrun bool) logrus.FieldLogger {
	logger := logrus.New()
	logger.SetFormatter(new(jobLogFormatter))
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.GetLevel())
	rtn := logger.WithFields(logrus.Fields{"job_name": jobName, "dryrun": dryrun})
	return rtn
}

type jobLogFormatter struct {
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
	var levelColor int
	switch entry.Level {
	case logrus.DebugLevel, logrus.TraceLevel:
		levelColor = gray
	case logrus.WarnLevel:
		levelColor = yellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		levelColor = red
	default:
		levelColor = blue
	}

	entry.Message = strings.TrimSuffix(entry.Message, "\n")
	jobName := entry.Data["job_name"]

	if entry.Data["dryrun"] == true {
		fmt.Fprintf(b, "\x1b[%dm*DRYRUN* \x1b[%dm[%s] \x1b[0m%s", green, levelColor, jobName, entry.Message)
	} else {
		fmt.Fprintf(b, "\x1b[%dm[%s] \x1b[0m%s", levelColor, jobName, entry.Message)
	}
}

func (f *jobLogFormatter) print(b *bytes.Buffer, entry *logrus.Entry) {
	entry.Message = strings.TrimSuffix(entry.Message, "\n")
	jobName := entry.Data["job_name"]

	if entry.Data["dryrun"] == true {
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
		return terminal.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}
