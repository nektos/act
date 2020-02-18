package otto

import (
	"errors"
	"fmt"

	"github.com/robertkrimen/otto/file"
)

type _exception struct {
	value interface{}
}

func newException(value interface{}) *_exception {
	return &_exception{
		value: value,
	}
}

func (self *_exception) eject() interface{} {
	value := self.value
	self.value = nil // Prevent Go from holding on to the value, whatever it is
	return value
}

type _error struct {
	name    string
	message string
	trace   []_frame

	offset int
}

func (err _error) format() string {
	if len(err.name) == 0 {
		return err.message
	}
	if len(err.message) == 0 {
		return err.name
	}
	return fmt.Sprintf("%s: %s", err.name, err.message)
}

func (err _error) formatWithStack() string {
	str := err.format() + "\n"
	for _, frame := range err.trace {
		str += "    at " + frame.location() + "\n"
	}
	return str
}

type _frame struct {
	native     bool
	nativeFile string
	nativeLine int
	file       *file.File
	offset     int
	callee     string
	fn         interface{}
}

var (
	nativeFrame = _frame{}
)

type _at int

func (fr _frame) location() string {
	str := "<unknown>"

	switch {
	case fr.native:
		str = "<native code>"
		if fr.nativeFile != "" && fr.nativeLine != 0 {
			str = fmt.Sprintf("%s:%d", fr.nativeFile, fr.nativeLine)
		}
	case fr.file != nil:
		if p := fr.file.Position(file.Idx(fr.offset)); p != nil {
			path, line, column := p.Filename, p.Line, p.Column

			if path == "" {
				path = "<anonymous>"
			}

			str = fmt.Sprintf("%s:%d:%d", path, line, column)
		}
	}

	if fr.callee != "" {
		str = fmt.Sprintf("%s (%s)", fr.callee, str)
	}

	return str
}

// An Error represents a runtime error, e.g. a TypeError, a ReferenceError, etc.
type Error struct {
	_error
}

// Error returns a description of the error
//
//    TypeError: 'def' is not a function
//
func (err Error) Error() string {
	return err.format()
}

// String returns a description of the error and a trace of where the
// error occurred.
//
//    TypeError: 'def' is not a function
//        at xyz (<anonymous>:3:9)
//        at <anonymous>:7:1/
//
func (err Error) String() string {
	return err.formatWithStack()
}

func (err _error) describe(format string, in ...interface{}) string {
	return fmt.Sprintf(format, in...)
}

func (self _error) messageValue() Value {
	if self.message == "" {
		return Value{}
	}
	return toValue_string(self.message)
}

func (rt *_runtime) typeErrorResult(throw bool) bool {
	if throw {
		panic(rt.panicTypeError())
	}
	return false
}

func newError(rt *_runtime, name string, stackFramesToPop int, in ...interface{}) _error {
	err := _error{
		name:   name,
		offset: -1,
	}
	description := ""
	length := len(in)

	if rt != nil && rt.scope != nil {
		scope := rt.scope

		for i := 0; i < stackFramesToPop; i++ {
			if scope.outer != nil {
				scope = scope.outer
			}
		}

		frame := scope.frame

		if length > 0 {
			if at, ok := in[length-1].(_at); ok {
				in = in[0 : length-1]
				if scope != nil {
					frame.offset = int(at)
				}
				length--
			}
			if length > 0 {
				description, in = in[0].(string), in[1:]
			}
		}

		limit := rt.traceLimit

		err.trace = append(err.trace, frame)
		if scope != nil {
			for scope = scope.outer; scope != nil; scope = scope.outer {
				if limit--; limit == 0 {
					break
				}

				if scope.frame.offset >= 0 {
					err.trace = append(err.trace, scope.frame)
				}
			}
		}
	} else {
		if length > 0 {
			description, in = in[0].(string), in[1:]
		}
	}
	err.message = err.describe(description, in...)

	return err
}

func (rt *_runtime) panicTypeError(argumentList ...interface{}) *_exception {
	return &_exception{
		value: newError(rt, "TypeError", 0, argumentList...),
	}
}

func (rt *_runtime) panicReferenceError(argumentList ...interface{}) *_exception {
	return &_exception{
		value: newError(rt, "ReferenceError", 0, argumentList...),
	}
}

func (rt *_runtime) panicURIError(argumentList ...interface{}) *_exception {
	return &_exception{
		value: newError(rt, "URIError", 0, argumentList...),
	}
}

func (rt *_runtime) panicSyntaxError(argumentList ...interface{}) *_exception {
	return &_exception{
		value: newError(rt, "SyntaxError", 0, argumentList...),
	}
}

func (rt *_runtime) panicRangeError(argumentList ...interface{}) *_exception {
	return &_exception{
		value: newError(rt, "RangeError", 0, argumentList...),
	}
}

func catchPanic(function func()) (err error) {
	defer func() {
		if caught := recover(); caught != nil {
			if exception, ok := caught.(*_exception); ok {
				caught = exception.eject()
			}
			switch caught := caught.(type) {
			case *Error:
				err = caught
				return
			case _error:
				err = &Error{caught}
				return
			case Value:
				if vl := caught._object(); vl != nil {
					switch vl := vl.value.(type) {
					case _error:
						err = &Error{vl}
						return
					}
				}
				err = errors.New(caught.string())
				return
			}
			panic(caught)
		}
	}()
	function()
	return nil
}
