/*Package assert provides assertions for comparing expected values to actual
values. When an assertion fails a helpful error message is printed.

Assert and Check

Assert() and Check() both accept a Comparison, and fail the test when the
comparison fails. The one difference is that Assert() will end the test execution
immediately (using t.FailNow()) whereas Check() will fail the test (using t.Fail()),
return the value of the comparison, then proceed with the rest of the test case.

Example usage

The example below shows assert used with some common types.


	import (
	    "testing"

	    "gotest.tools/assert"
	    is "gotest.tools/assert/cmp"
	)

	func TestEverything(t *testing.T) {
	    // booleans
	    assert.Assert(t, ok)
	    assert.Assert(t, !missing)

	    // primitives
	    assert.Equal(t, count, 1)
	    assert.Equal(t, msg, "the message")
	    assert.Assert(t, total != 10) // NotEqual

	    // errors
	    assert.NilError(t, closer.Close())
	    assert.Error(t, err, "the exact error message")
	    assert.ErrorContains(t, err, "includes this")
	    assert.ErrorType(t, err, os.IsNotExist)

	    // complex types
	    assert.DeepEqual(t, result, myStruct{Name: "title"})
	    assert.Assert(t, is.Len(items, 3))
	    assert.Assert(t, len(sequence) != 0) // NotEmpty
	    assert.Assert(t, is.Contains(mapping, "key"))

	    // pointers and interface
	    assert.Assert(t, is.Nil(ref))
	    assert.Assert(t, ref != nil) // NotNil
	}

Comparisons

Package https://godoc.org/gotest.tools/assert/cmp provides
many common comparisons. Additional comparisons can be written to compare
values in other ways. See the example Assert (CustomComparison).

Automated migration from testify

gty-migrate-from-testify is a binary which can update source code which uses
testify assertions to use the assertions provided by this package.

See http://bit.do/cmd-gty-migrate-from-testify.


*/
package assert // import "gotest.tools/assert"

import (
	"fmt"
	"go/ast"
	"go/token"

	gocmp "github.com/google/go-cmp/cmp"
	"gotest.tools/assert/cmp"
	"gotest.tools/internal/format"
	"gotest.tools/internal/source"
)

// BoolOrComparison can be a bool, or cmp.Comparison. See Assert() for usage.
type BoolOrComparison interface{}

// TestingT is the subset of testing.T used by the assert package.
type TestingT interface {
	FailNow()
	Fail()
	Log(args ...interface{})
}

type helperT interface {
	Helper()
}

const failureMessage = "assertion failed: "

// nolint: gocyclo
func assert(
	t TestingT,
	failer func(),
	argSelector argSelector,
	comparison BoolOrComparison,
	msgAndArgs ...interface{},
) bool {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	var success bool
	switch check := comparison.(type) {
	case bool:
		if check {
			return true
		}
		logFailureFromBool(t, msgAndArgs...)

	// Undocumented legacy comparison without Result type
	case func() (success bool, message string):
		success = runCompareFunc(t, check, msgAndArgs...)

	case nil:
		return true

	case error:
		msg := "error is not nil: "
		t.Log(format.WithCustomMessage(failureMessage+msg+check.Error(), msgAndArgs...))

	case cmp.Comparison:
		success = runComparison(t, argSelector, check, msgAndArgs...)

	case func() cmp.Result:
		success = runComparison(t, argSelector, check, msgAndArgs...)

	default:
		t.Log(fmt.Sprintf("invalid Comparison: %v (%T)", check, check))
	}

	if success {
		return true
	}
	failer()
	return false
}

func runCompareFunc(
	t TestingT,
	f func() (success bool, message string),
	msgAndArgs ...interface{},
) bool {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if success, message := f(); !success {
		t.Log(format.WithCustomMessage(failureMessage+message, msgAndArgs...))
		return false
	}
	return true
}

func logFailureFromBool(t TestingT, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	const stackIndex = 3 // Assert()/Check(), assert(), formatFailureFromBool()
	const comparisonArgPos = 1
	args, err := source.CallExprArgs(stackIndex)
	if err != nil {
		t.Log(err.Error())
		return
	}

	msg, err := boolFailureMessage(args[comparisonArgPos])
	if err != nil {
		t.Log(err.Error())
		msg = "expression is false"
	}

	t.Log(format.WithCustomMessage(failureMessage+msg, msgAndArgs...))
}

func boolFailureMessage(expr ast.Expr) (string, error) {
	if binaryExpr, ok := expr.(*ast.BinaryExpr); ok && binaryExpr.Op == token.NEQ {
		x, err := source.FormatNode(binaryExpr.X)
		if err != nil {
			return "", err
		}
		y, err := source.FormatNode(binaryExpr.Y)
		if err != nil {
			return "", err
		}
		return x + " is " + y, nil
	}

	if unaryExpr, ok := expr.(*ast.UnaryExpr); ok && unaryExpr.Op == token.NOT {
		x, err := source.FormatNode(unaryExpr.X)
		if err != nil {
			return "", err
		}
		return x + " is true", nil
	}

	formatted, err := source.FormatNode(expr)
	if err != nil {
		return "", err
	}
	return "expression is false: " + formatted, nil
}

// Assert performs a comparison. If the comparison fails the test is marked as
// failed, a failure message is logged, and execution is stopped immediately.
//
// The comparison argument may be one of three types: bool, cmp.Comparison or
// error.
// When called with a bool the failure message will contain the literal source
// code of the expression.
// When called with a cmp.Comparison the comparison is responsible for producing
// a helpful failure message.
// When called with an error a nil value is considered success. A non-nil error
// is a failure, and Error() is used as the failure message.
func Assert(t TestingT, comparison BoolOrComparison, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert(t, t.FailNow, argsFromComparisonCall, comparison, msgAndArgs...)
}

// Check performs a comparison. If the comparison fails the test is marked as
// failed, a failure message is logged, and Check returns false. Otherwise returns
// true.
//
// See Assert for details about the comparison arg and failure messages.
func Check(t TestingT, comparison BoolOrComparison, msgAndArgs ...interface{}) bool {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	return assert(t, t.Fail, argsFromComparisonCall, comparison, msgAndArgs...)
}

// NilError fails the test immediately if err is not nil.
// This is equivalent to Assert(t, err)
func NilError(t TestingT, err error, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert(t, t.FailNow, argsAfterT, err, msgAndArgs...)
}

// Equal uses the == operator to assert two values are equal and fails the test
// if they are not equal.
//
// If the comparison fails Equal will use the variable names for x and y as part
// of the failure message to identify the actual and expected values.
//
// If either x or y are a multi-line string the failure message will include a
// unified diff of the two values. If the values only differ by whitespace
// the unified diff will be augmented by replacing whitespace characters with
// visible characters to identify the whitespace difference.
//
// This is equivalent to Assert(t, cmp.Equal(x, y)).
func Equal(t TestingT, x, y interface{}, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert(t, t.FailNow, argsAfterT, cmp.Equal(x, y), msgAndArgs...)
}

// DeepEqual uses google/go-cmp (http://bit.do/go-cmp) to assert two values are
// equal and fails the test if they are not equal.
//
// Package https://godoc.org/gotest.tools/assert/opt provides some additional
// commonly used Options.
//
// This is equivalent to Assert(t, cmp.DeepEqual(x, y)).
func DeepEqual(t TestingT, x, y interface{}, opts ...gocmp.Option) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert(t, t.FailNow, argsAfterT, cmp.DeepEqual(x, y, opts...))
}

// Error fails the test if err is nil, or the error message is not the expected
// message.
// Equivalent to Assert(t, cmp.Error(err, message)).
func Error(t TestingT, err error, message string, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert(t, t.FailNow, argsAfterT, cmp.Error(err, message), msgAndArgs...)
}

// ErrorContains fails the test if err is nil, or the error message does not
// contain the expected substring.
// Equivalent to Assert(t, cmp.ErrorContains(err, substring)).
func ErrorContains(t TestingT, err error, substring string, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert(t, t.FailNow, argsAfterT, cmp.ErrorContains(err, substring), msgAndArgs...)
}

// ErrorType fails the test if err is nil, or err is not the expected type.
//
// Expected can be one of:
// a func(error) bool which returns true if the error is the expected type,
// an instance of (or a pointer to) a struct of the expected type,
// a pointer to an interface the error is expected to implement,
// a reflect.Type of the expected struct or interface.
//
// Equivalent to Assert(t, cmp.ErrorType(err, expected)).
func ErrorType(t TestingT, err error, expected interface{}, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert(t, t.FailNow, argsAfterT, cmp.ErrorType(err, expected), msgAndArgs...)
}
