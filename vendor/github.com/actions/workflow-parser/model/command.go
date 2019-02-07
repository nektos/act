package model

import (
	"strings"
)

// Command represents the optional "runs" and "args" attributes.
// Each one takes one of two forms:
//   - runs="entrypoint arg1 arg2 ..."
//   - runs=[ "entrypoint", "arg1", "arg2", ... ]
type Command interface {
	isCommand()
	Split() []string
}

// StringCommand represents the string based form of the "runs" or "args"
// attribute.
//   - runs="entrypoint arg1 arg2 ..."
type StringCommand struct {
	Value string
}

// ListCommand represents the list based form of the "runs" or "args" attribute.
//   - runs=[ "entrypoint", "arg1", "arg2", ... ]
type ListCommand struct {
	Values []string
}

func (s *StringCommand) isCommand() {}
func (l *ListCommand) isCommand()   {}

func (s *StringCommand) Split() []string {
	return strings.Fields(s.Value)
}

func (l *ListCommand) Split() []string {
	return l.Values
}
