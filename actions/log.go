package actions

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

type actionLogFormatter struct {
}

var formatter *actionLogFormatter

func init() {
	formatter = new(actionLogFormatter)
}

const (
	//nocolor = 0
	red    = 31
	green  = 32
	yellow = 33
	blue   = 36
	gray   = 37
)

func newActionLogger(actionName string, dryrun bool) *logrus.Entry {
	logger := logrus.New()
	logger.SetFormatter(formatter)
	logger.SetLevel(logrus.GetLevel())
	rtn := logger.WithFields(logrus.Fields{"action_name": actionName, "dryrun": dryrun})
	return rtn
}

func (f *actionLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	if f.isColored(entry) {
		f.printColored(b, entry)
	} else {
		f.print(b, entry)
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *actionLogFormatter) printColored(b *bytes.Buffer, entry *logrus.Entry) {
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
	actionName := entry.Data["action_name"]

	if entry.Data["dryrun"] == true {
		fmt.Fprintf(b, "\x1b[%dm*DRYRUN* \x1b[%dm[%s] \x1b[0m%s", green, levelColor, actionName, entry.Message)
	} else {
		fmt.Fprintf(b, "\x1b[%dm[%s] \x1b[0m%s", levelColor, actionName, entry.Message)
	}
}

func (f *actionLogFormatter) print(b *bytes.Buffer, entry *logrus.Entry) {
	entry.Message = strings.TrimSuffix(entry.Message, "\n")
	actionName := entry.Data["action_name"]

	if entry.Data["dryrun"] == true {
		fmt.Fprintf(b, "*DRYRUN* [%s] %s", actionName, entry.Message)
	} else {
		fmt.Fprintf(b, "[%s] %s", actionName, entry.Message)
	}
}

func (f *actionLogFormatter) isColored(entry *logrus.Entry) bool {

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
