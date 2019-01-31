package parser

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/actions/workflow-parser/model"
)

type ParserError struct {
	message   string
	Errors    ErrorList
	Actions   []*model.Action
	Workflows []*model.Workflow
}

func (p *ParserError) Error() string {
	return p.message
}

// Error represents an error identified by the parser, either syntactic
// (HCL) or semantic (.workflow) in nature.  There are fields for location
// (File, Line, Column), severity, and base error string.  The `Error()`
// function on this type concatenates whatever bits of the location are
// available with the message.  The severity is only used for filtering.
type Error struct {
	message  string
	Pos      ErrorPos
	Severity Severity
}

// ErrorPos represents the location of an error in a user's workflow
// file(s).
type ErrorPos struct {
	File   string
	Line   int
	Column int
}

// newFatal creates a new error at the FATAL level, indicating that the
// file is so broken it should not be displayed.
func newFatal(pos ErrorPos, format string, a ...interface{}) *Error {
	return &Error{
		message:  fmt.Sprintf(format, a...),
		Pos:      pos,
		Severity: FATAL,
	}
}

// newError creates a new error at the ERROR level, indicating that the
// file can be displayed but cannot be run.
func newError(pos ErrorPos, format string, a ...interface{}) *Error {
	return &Error{
		message:  fmt.Sprintf(format, a...),
		Pos:      pos,
		Severity: ERROR,
	}
}

// newWarning creates a new error at the WARNING level, indicating that
// the file might be runnable but might not execute as intended.
func newWarning(pos ErrorPos, format string, a ...interface{}) *Error {
	return &Error{
		message:  fmt.Sprintf(format, a...),
		Pos:      pos,
		Severity: WARNING,
	}
}

func (e *Error) Error() string {
	var sb strings.Builder
	if e.Pos.Line != 0 {
		sb.WriteString("Line ")                  // nolint: errcheck
		sb.WriteString(strconv.Itoa(e.Pos.Line)) // nolint: errcheck
		sb.WriteString(": ")                     // nolint: errcheck
	}
	if sb.Len() > 0 {
		sb.WriteString(e.message) // nolint: errcheck
		return sb.String()
	}
	return e.message
}

const (
	_ = iota

	// WARNING indicates a mistake that might affect correctness
	WARNING

	// ERROR indicates a mistake that prevents execution of any workflows in the file
	ERROR

	// FATAL indicates a mistake that prevents even drawing the file
	FATAL
)

// Severity represents the level of an error encountered while parsing a
// workflow file.  See the comments for WARNING, ERROR, and FATAL, above.
type Severity int

// FirstError searches a Configuration for the first error at or above a
// given severity level.  Checking the return value against nil is a good
// way to see if the file has any errors at or above the given severity.
// A caller intending to execute the file might check for
// `errors.FirstError(parser.WARNING)`, while a caller intending to
// display the file might check for `errors.FirstError(parser.FATAL)`.
func (errors ErrorList) FirstError(severity Severity) error {
	for _, e := range errors {
		if e.Severity >= severity {
			return e
		}
	}
	return nil
}

type ErrorList []*Error

func (a ErrorList) Len() int           { return len(a) }
func (a ErrorList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ErrorList) Less(i, j int) bool { return a[i].Pos.Line < a[j].Pos.Line }

// sortErrors sorts the errors reported by the parser.  Do this after
// parsing is complete.  The sort is stable, so order is preserved within
// a single line: left to right, syntax errors before validation errors.
func (errors ErrorList) sort() {
	sort.Stable(errors)
}
