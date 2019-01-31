package assert

import (
	"fmt"
	"go/ast"

	"gotest.tools/assert/cmp"
	"gotest.tools/internal/format"
	"gotest.tools/internal/source"
)

func runComparison(
	t TestingT,
	argSelector argSelector,
	f cmp.Comparison,
	msgAndArgs ...interface{},
) bool {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	result := f()
	if result.Success() {
		return true
	}

	var message string
	switch typed := result.(type) {
	case resultWithComparisonArgs:
		const stackIndex = 3 // Assert/Check, assert, runComparison
		args, err := source.CallExprArgs(stackIndex)
		if err != nil {
			t.Log(err.Error())
		}
		message = typed.FailureMessage(filterPrintableExpr(argSelector(args)))
	case resultBasic:
		message = typed.FailureMessage()
	default:
		message = fmt.Sprintf("comparison returned invalid Result type: %T", result)
	}

	t.Log(format.WithCustomMessage(failureMessage+message, msgAndArgs...))
	return false
}

type resultWithComparisonArgs interface {
	FailureMessage(args []ast.Expr) string
}

type resultBasic interface {
	FailureMessage() string
}

// filterPrintableExpr filters the ast.Expr slice to only include Expr that are
// easy to read when printed and contain relevant information to an assertion.
//
// Ident and SelectorExpr are included because they print nicely and the variable
// names may provide additional context to their values.
// BasicLit and CompositeLit are excluded because their source is equivalent to
// their value, which is already available.
// Other types are ignored for now, but could be added if they are relevant.
func filterPrintableExpr(args []ast.Expr) []ast.Expr {
	result := make([]ast.Expr, len(args))
	for i, arg := range args {
		if isShortPrintableExpr(arg) {
			result[i] = arg
			continue
		}

		if starExpr, ok := arg.(*ast.StarExpr); ok {
			result[i] = starExpr.X
			continue
		}
	}
	return result
}

func isShortPrintableExpr(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.SliceExpr:
		return true
	case *ast.BinaryExpr, *ast.UnaryExpr:
		return true
	default:
		// CallExpr, ParenExpr, TypeAssertExpr, KeyValueExpr, StarExpr
		return false
	}
}

type argSelector func([]ast.Expr) []ast.Expr

func argsAfterT(args []ast.Expr) []ast.Expr {
	if len(args) < 1 {
		return nil
	}
	return args[1:]
}

func argsFromComparisonCall(args []ast.Expr) []ast.Expr {
	if len(args) < 1 {
		return nil
	}
	if callExpr, ok := args[1].(*ast.CallExpr); ok {
		return callExpr.Args
	}
	return nil
}
