// Copyright 2017, The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package cmp determines equality of values.
//
// This package is intended to be a more powerful and safer alternative to
// reflect.DeepEqual for comparing whether two values are semantically equal.
//
// The primary features of cmp are:
//
// • When the default behavior of equality does not suit the needs of the test,
// custom equality functions can override the equality operation.
// For example, an equality function may report floats as equal so long as they
// are within some tolerance of each other.
//
// • Types that have an Equal method may use that method to determine equality.
// This allows package authors to determine the equality operation for the types
// that they define.
//
// • If no custom equality functions are used and no Equal method is defined,
// equality is determined by recursively comparing the primitive kinds on both
// values, much like reflect.DeepEqual. Unlike reflect.DeepEqual, unexported
// fields are not compared by default; they result in panics unless suppressed
// by using an Ignore option (see cmpopts.IgnoreUnexported) or explicitly compared
// using the AllowUnexported option.
package cmp

import (
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp/internal/diff"
	"github.com/google/go-cmp/cmp/internal/function"
	"github.com/google/go-cmp/cmp/internal/value"
)

// BUG(dsnet): Maps with keys containing NaN values cannot be properly compared due to
// the reflection package's inability to retrieve such entries. Equal will panic
// anytime it comes across a NaN key, but this behavior may change.
//
// See https://golang.org/issue/11104 for more details.

var nothing = reflect.Value{}

// Equal reports whether x and y are equal by recursively applying the
// following rules in the given order to x and y and all of their sub-values:
//
// • If two values are not of the same type, then they are never equal
// and the overall result is false.
//
// • Let S be the set of all Ignore, Transformer, and Comparer options that
// remain after applying all path filters, value filters, and type filters.
// If at least one Ignore exists in S, then the comparison is ignored.
// If the number of Transformer and Comparer options in S is greater than one,
// then Equal panics because it is ambiguous which option to use.
// If S contains a single Transformer, then use that to transform the current
// values and recursively call Equal on the output values.
// If S contains a single Comparer, then use that to compare the current values.
// Otherwise, evaluation proceeds to the next rule.
//
// • If the values have an Equal method of the form "(T) Equal(T) bool" or
// "(T) Equal(I) bool" where T is assignable to I, then use the result of
// x.Equal(y) even if x or y is nil.
// Otherwise, no such method exists and evaluation proceeds to the next rule.
//
// • Lastly, try to compare x and y based on their basic kinds.
// Simple kinds like booleans, integers, floats, complex numbers, strings, and
// channels are compared using the equivalent of the == operator in Go.
// Functions are only equal if they are both nil, otherwise they are unequal.
// Pointers are equal if the underlying values they point to are also equal.
// Interfaces are equal if their underlying concrete values are also equal.
//
// Structs are equal if all of their fields are equal. If a struct contains
// unexported fields, Equal panics unless the AllowUnexported option is used or
// an Ignore option (e.g., cmpopts.IgnoreUnexported) ignores that field.
//
// Arrays, slices, and maps are equal if they are both nil or both non-nil
// with the same length and the elements at each index or key are equal.
// Note that a non-nil empty slice and a nil slice are not equal.
// To equate empty slices and maps, consider using cmpopts.EquateEmpty.
// Map keys are equal according to the == operator.
// To use custom comparisons for map keys, consider using cmpopts.SortMaps.
func Equal(x, y interface{}, opts ...Option) bool {
	s := newState(opts)
	s.compareAny(reflect.ValueOf(x), reflect.ValueOf(y))
	return s.result.Equal()
}

// Diff returns a human-readable report of the differences between two values.
// It returns an empty string if and only if Equal returns true for the same
// input values and options. The output string will use the "-" symbol to
// indicate elements removed from x, and the "+" symbol to indicate elements
// added to y.
//
// Do not depend on this output being stable.
func Diff(x, y interface{}, opts ...Option) string {
	r := new(defaultReporter)
	opts = Options{Options(opts), r}
	eq := Equal(x, y, opts...)
	d := r.String()
	if (d == "") != eq {
		panic("inconsistent difference and equality results")
	}
	return d
}

type state struct {
	// These fields represent the "comparison state".
	// Calling statelessCompare must not result in observable changes to these.
	result   diff.Result // The current result of comparison
	curPath  Path        // The current path in the value tree
	reporter reporter    // Optional reporter used for difference formatting

	// dynChecker triggers pseudo-random checks for option correctness.
	// It is safe for statelessCompare to mutate this value.
	dynChecker dynChecker

	// These fields, once set by processOption, will not change.
	exporters map[reflect.Type]bool // Set of structs with unexported field visibility
	opts      Options               // List of all fundamental and filter options
}

func newState(opts []Option) *state {
	s := new(state)
	for _, opt := range opts {
		s.processOption(opt)
	}
	return s
}

func (s *state) processOption(opt Option) {
	switch opt := opt.(type) {
	case nil:
	case Options:
		for _, o := range opt {
			s.processOption(o)
		}
	case coreOption:
		type filtered interface {
			isFiltered() bool
		}
		if fopt, ok := opt.(filtered); ok && !fopt.isFiltered() {
			panic(fmt.Sprintf("cannot use an unfiltered option: %v", opt))
		}
		s.opts = append(s.opts, opt)
	case visibleStructs:
		if s.exporters == nil {
			s.exporters = make(map[reflect.Type]bool)
		}
		for t := range opt {
			s.exporters[t] = true
		}
	case reporter:
		if s.reporter != nil {
			panic("difference reporter already registered")
		}
		s.reporter = opt
	default:
		panic(fmt.Sprintf("unknown option %T", opt))
	}
}

// statelessCompare compares two values and returns the result.
// This function is stateless in that it does not alter the current result,
// or output to any registered reporters.
func (s *state) statelessCompare(vx, vy reflect.Value) diff.Result {
	// We do not save and restore the curPath because all of the compareX
	// methods should properly push and pop from the path.
	// It is an implementation bug if the contents of curPath differs from
	// when calling this function to when returning from it.

	oldResult, oldReporter := s.result, s.reporter
	s.result = diff.Result{} // Reset result
	s.reporter = nil         // Remove reporter to avoid spurious printouts
	s.compareAny(vx, vy)
	res := s.result
	s.result, s.reporter = oldResult, oldReporter
	return res
}

func (s *state) compareAny(vx, vy reflect.Value) {
	// TODO: Support cyclic data structures.

	// Rule 0: Differing types are never equal.
	if !vx.IsValid() || !vy.IsValid() {
		s.report(vx.IsValid() == vy.IsValid(), vx, vy)
		return
	}
	if vx.Type() != vy.Type() {
		s.report(false, vx, vy) // Possible for path to be empty
		return
	}
	t := vx.Type()
	if len(s.curPath) == 0 {
		s.curPath.push(&pathStep{typ: t})
		defer s.curPath.pop()
	}
	vx, vy = s.tryExporting(vx, vy)

	// Rule 1: Check whether an option applies on this node in the value tree.
	if s.tryOptions(vx, vy, t) {
		return
	}

	// Rule 2: Check whether the type has a valid Equal method.
	if s.tryMethod(vx, vy, t) {
		return
	}

	// Rule 3: Recursively descend into each value's underlying kind.
	switch t.Kind() {
	case reflect.Bool:
		s.report(vx.Bool() == vy.Bool(), vx, vy)
		return
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s.report(vx.Int() == vy.Int(), vx, vy)
		return
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		s.report(vx.Uint() == vy.Uint(), vx, vy)
		return
	case reflect.Float32, reflect.Float64:
		s.report(vx.Float() == vy.Float(), vx, vy)
		return
	case reflect.Complex64, reflect.Complex128:
		s.report(vx.Complex() == vy.Complex(), vx, vy)
		return
	case reflect.String:
		s.report(vx.String() == vy.String(), vx, vy)
		return
	case reflect.Chan, reflect.UnsafePointer:
		s.report(vx.Pointer() == vy.Pointer(), vx, vy)
		return
	case reflect.Func:
		s.report(vx.IsNil() && vy.IsNil(), vx, vy)
		return
	case reflect.Ptr:
		if vx.IsNil() || vy.IsNil() {
			s.report(vx.IsNil() && vy.IsNil(), vx, vy)
			return
		}
		s.curPath.push(&indirect{pathStep{t.Elem()}})
		defer s.curPath.pop()
		s.compareAny(vx.Elem(), vy.Elem())
		return
	case reflect.Interface:
		if vx.IsNil() || vy.IsNil() {
			s.report(vx.IsNil() && vy.IsNil(), vx, vy)
			return
		}
		if vx.Elem().Type() != vy.Elem().Type() {
			s.report(false, vx.Elem(), vy.Elem())
			return
		}
		s.curPath.push(&typeAssertion{pathStep{vx.Elem().Type()}})
		defer s.curPath.pop()
		s.compareAny(vx.Elem(), vy.Elem())
		return
	case reflect.Slice:
		if vx.IsNil() || vy.IsNil() {
			s.report(vx.IsNil() && vy.IsNil(), vx, vy)
			return
		}
		fallthrough
	case reflect.Array:
		s.compareArray(vx, vy, t)
		return
	case reflect.Map:
		s.compareMap(vx, vy, t)
		return
	case reflect.Struct:
		s.compareStruct(vx, vy, t)
		return
	default:
		panic(fmt.Sprintf("%v kind not handled", t.Kind()))
	}
}

func (s *state) tryExporting(vx, vy reflect.Value) (reflect.Value, reflect.Value) {
	if sf, ok := s.curPath[len(s.curPath)-1].(*structField); ok && sf.unexported {
		if sf.force {
			// Use unsafe pointer arithmetic to get read-write access to an
			// unexported field in the struct.
			vx = unsafeRetrieveField(sf.pvx, sf.field)
			vy = unsafeRetrieveField(sf.pvy, sf.field)
		} else {
			// We are not allowed to export the value, so invalidate them
			// so that tryOptions can panic later if not explicitly ignored.
			vx = nothing
			vy = nothing
		}
	}
	return vx, vy
}

func (s *state) tryOptions(vx, vy reflect.Value, t reflect.Type) bool {
	// If there were no FilterValues, we will not detect invalid inputs,
	// so manually check for them and append invalid if necessary.
	// We still evaluate the options since an ignore can override invalid.
	opts := s.opts
	if !vx.IsValid() || !vy.IsValid() {
		opts = Options{opts, invalid{}}
	}

	// Evaluate all filters and apply the remaining options.
	if opt := opts.filter(s, vx, vy, t); opt != nil {
		opt.apply(s, vx, vy)
		return true
	}
	return false
}

func (s *state) tryMethod(vx, vy reflect.Value, t reflect.Type) bool {
	// Check if this type even has an Equal method.
	m, ok := t.MethodByName("Equal")
	if !ok || !function.IsType(m.Type, function.EqualAssignable) {
		return false
	}

	eq := s.callTTBFunc(m.Func, vx, vy)
	s.report(eq, vx, vy)
	return true
}

func (s *state) callTRFunc(f, v reflect.Value) reflect.Value {
	v = sanitizeValue(v, f.Type().In(0))
	if !s.dynChecker.Next() {
		return f.Call([]reflect.Value{v})[0]
	}

	// Run the function twice and ensure that we get the same results back.
	// We run in goroutines so that the race detector (if enabled) can detect
	// unsafe mutations to the input.
	c := make(chan reflect.Value)
	go detectRaces(c, f, v)
	want := f.Call([]reflect.Value{v})[0]
	if got := <-c; !s.statelessCompare(got, want).Equal() {
		// To avoid false-positives with non-reflexive equality operations,
		// we sanity check whether a value is equal to itself.
		if !s.statelessCompare(want, want).Equal() {
			return want
		}
		fn := getFuncName(f.Pointer())
		panic(fmt.Sprintf("non-deterministic function detected: %s", fn))
	}
	return want
}

func (s *state) callTTBFunc(f, x, y reflect.Value) bool {
	x = sanitizeValue(x, f.Type().In(0))
	y = sanitizeValue(y, f.Type().In(1))
	if !s.dynChecker.Next() {
		return f.Call([]reflect.Value{x, y})[0].Bool()
	}

	// Swapping the input arguments is sufficient to check that
	// f is symmetric and deterministic.
	// We run in goroutines so that the race detector (if enabled) can detect
	// unsafe mutations to the input.
	c := make(chan reflect.Value)
	go detectRaces(c, f, y, x)
	want := f.Call([]reflect.Value{x, y})[0].Bool()
	if got := <-c; !got.IsValid() || got.Bool() != want {
		fn := getFuncName(f.Pointer())
		panic(fmt.Sprintf("non-deterministic or non-symmetric function detected: %s", fn))
	}
	return want
}

func detectRaces(c chan<- reflect.Value, f reflect.Value, vs ...reflect.Value) {
	var ret reflect.Value
	defer func() {
		recover() // Ignore panics, let the other call to f panic instead
		c <- ret
	}()
	ret = f.Call(vs)[0]
}

// sanitizeValue converts nil interfaces of type T to those of type R,
// assuming that T is assignable to R.
// Otherwise, it returns the input value as is.
func sanitizeValue(v reflect.Value, t reflect.Type) reflect.Value {
	// TODO(dsnet): Remove this hacky workaround.
	// See https://golang.org/issue/22143
	if v.Kind() == reflect.Interface && v.IsNil() && v.Type() != t {
		return reflect.New(t).Elem()
	}
	return v
}

func (s *state) compareArray(vx, vy reflect.Value, t reflect.Type) {
	step := &sliceIndex{pathStep{t.Elem()}, 0, 0}
	s.curPath.push(step)

	// Compute an edit-script for slices vx and vy.
	es := diff.Difference(vx.Len(), vy.Len(), func(ix, iy int) diff.Result {
		step.xkey, step.ykey = ix, iy
		return s.statelessCompare(vx.Index(ix), vy.Index(iy))
	})

	// Report the entire slice as is if the arrays are of primitive kind,
	// and the arrays are different enough.
	isPrimitive := false
	switch t.Elem().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Bool, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		isPrimitive = true
	}
	if isPrimitive && es.Dist() > (vx.Len()+vy.Len())/4 {
		s.curPath.pop() // Pop first since we are reporting the whole slice
		s.report(false, vx, vy)
		return
	}

	// Replay the edit-script.
	var ix, iy int
	for _, e := range es {
		switch e {
		case diff.UniqueX:
			step.xkey, step.ykey = ix, -1
			s.report(false, vx.Index(ix), nothing)
			ix++
		case diff.UniqueY:
			step.xkey, step.ykey = -1, iy
			s.report(false, nothing, vy.Index(iy))
			iy++
		default:
			step.xkey, step.ykey = ix, iy
			if e == diff.Identity {
				s.report(true, vx.Index(ix), vy.Index(iy))
			} else {
				s.compareAny(vx.Index(ix), vy.Index(iy))
			}
			ix++
			iy++
		}
	}
	s.curPath.pop()
	return
}

func (s *state) compareMap(vx, vy reflect.Value, t reflect.Type) {
	if vx.IsNil() || vy.IsNil() {
		s.report(vx.IsNil() && vy.IsNil(), vx, vy)
		return
	}

	// We combine and sort the two map keys so that we can perform the
	// comparisons in a deterministic order.
	step := &mapIndex{pathStep: pathStep{t.Elem()}}
	s.curPath.push(step)
	defer s.curPath.pop()
	for _, k := range value.SortKeys(append(vx.MapKeys(), vy.MapKeys()...)) {
		step.key = k
		vvx := vx.MapIndex(k)
		vvy := vy.MapIndex(k)
		switch {
		case vvx.IsValid() && vvy.IsValid():
			s.compareAny(vvx, vvy)
		case vvx.IsValid() && !vvy.IsValid():
			s.report(false, vvx, nothing)
		case !vvx.IsValid() && vvy.IsValid():
			s.report(false, nothing, vvy)
		default:
			// It is possible for both vvx and vvy to be invalid if the
			// key contained a NaN value in it. There is no way in
			// reflection to be able to retrieve these values.
			// See https://golang.org/issue/11104
			panic(fmt.Sprintf("%#v has map key with NaNs", s.curPath))
		}
	}
}

func (s *state) compareStruct(vx, vy reflect.Value, t reflect.Type) {
	var vax, vay reflect.Value // Addressable versions of vx and vy

	step := &structField{}
	s.curPath.push(step)
	defer s.curPath.pop()
	for i := 0; i < t.NumField(); i++ {
		vvx := vx.Field(i)
		vvy := vy.Field(i)
		step.typ = t.Field(i).Type
		step.name = t.Field(i).Name
		step.idx = i
		step.unexported = !isExported(step.name)
		if step.unexported {
			// Defer checking of unexported fields until later to give an
			// Ignore a chance to ignore the field.
			if !vax.IsValid() || !vay.IsValid() {
				// For unsafeRetrieveField to work, the parent struct must
				// be addressable. Create a new copy of the values if
				// necessary to make them addressable.
				vax = makeAddressable(vx)
				vay = makeAddressable(vy)
			}
			step.force = s.exporters[t]
			step.pvx = vax
			step.pvy = vay
			step.field = t.Field(i)
		}
		s.compareAny(vvx, vvy)
	}
}

// report records the result of a single comparison.
// It also calls Report if any reporter is registered.
func (s *state) report(eq bool, vx, vy reflect.Value) {
	if eq {
		s.result.NSame++
	} else {
		s.result.NDiff++
	}
	if s.reporter != nil {
		s.reporter.Report(vx, vy, eq, s.curPath)
	}
}

// dynChecker tracks the state needed to periodically perform checks that
// user provided functions are symmetric and deterministic.
// The zero value is safe for immediate use.
type dynChecker struct{ curr, next int }

// Next increments the state and reports whether a check should be performed.
//
// Checks occur every Nth function call, where N is a triangular number:
//	0 1 3 6 10 15 21 28 36 45 55 66 78 91 105 120 136 153 171 190 ...
// See https://en.wikipedia.org/wiki/Triangular_number
//
// This sequence ensures that the cost of checks drops significantly as
// the number of functions calls grows larger.
func (dc *dynChecker) Next() bool {
	ok := dc.curr == dc.next
	if ok {
		dc.curr = 0
		dc.next++
	}
	dc.curr++
	return ok
}

// makeAddressable returns a value that is always addressable.
// It returns the input verbatim if it is already addressable,
// otherwise it creates a new value and returns an addressable copy.
func makeAddressable(v reflect.Value) reflect.Value {
	if v.CanAddr() {
		return v
	}
	vc := reflect.New(v.Type()).Elem()
	vc.Set(v)
	return vc
}
