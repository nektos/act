package cmp

import (
	"bytes"
	"fmt"
	"go/ast"
	"text/template"

	"gotest.tools/internal/source"
)

// Result of a Comparison.
type Result interface {
	Success() bool
}

type result struct {
	success bool
	message string
}

func (r result) Success() bool {
	return r.success
}

func (r result) FailureMessage() string {
	return r.message
}

// ResultSuccess is a constant which is returned by a ComparisonWithResult to
// indicate success.
var ResultSuccess = result{success: true}

// ResultFailure returns a failed Result with a failure message.
func ResultFailure(message string) Result {
	return result{message: message}
}

// ResultFromError returns ResultSuccess if err is nil. Otherwise ResultFailure
// is returned with the error message as the failure message.
func ResultFromError(err error) Result {
	if err == nil {
		return ResultSuccess
	}
	return ResultFailure(err.Error())
}

type templatedResult struct {
	success  bool
	template string
	data     map[string]interface{}
}

func (r templatedResult) Success() bool {
	return r.success
}

func (r templatedResult) FailureMessage(args []ast.Expr) string {
	msg, err := renderMessage(r, args)
	if err != nil {
		return fmt.Sprintf("failed to render failure message: %s", err)
	}
	return msg
}

// ResultFailureTemplate returns a Result with a template string and data which
// can be used to format a failure message. The template may access data from .Data,
// the comparison args with the callArg function, and the formatNode function may
// be used to format the call args.
func ResultFailureTemplate(template string, data map[string]interface{}) Result {
	return templatedResult{template: template, data: data}
}

func renderMessage(result templatedResult, args []ast.Expr) (string, error) {
	tmpl := template.New("failure").Funcs(template.FuncMap{
		"formatNode": source.FormatNode,
		"callArg": func(index int) ast.Expr {
			if index >= len(args) {
				return nil
			}
			return args[index]
		},
	})
	var err error
	tmpl, err = tmpl.Parse(result.template)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, map[string]interface{}{
		"Data": result.data,
	})
	return buf.String(), err
}
