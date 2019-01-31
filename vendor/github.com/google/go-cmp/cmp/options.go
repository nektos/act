// Copyright 2017, The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package cmp

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/google/go-cmp/cmp/internal/function"
)

// Option configures for specific behavior of Equal and Diff. In particular,
// the fundamental Option functions (Ignore, Transformer, and Comparer),
// configure how equality is determined.
//
// The fundamental options may be composed with filters (FilterPath and
// FilterValues) to control the scope over which they are applied.
//
// The cmp/cmpopts package provides helper functions for creating options that
// may be used with Equal and Diff.
type Option interface {
	// filter applies all filters and returns the option that remains.
	// Each option may only read s.curPath and call s.callTTBFunc.
	//
	// An Options is returned only if multiple comparers or transformers
	// can apply simultaneously and will only contain values of those types
	// or sub-Options containing values of those types.
	filter(s *state, vx, vy reflect.Value, t reflect.Type) applicableOption
}

// applicableOption represents the following types:
//	Fundamental: ignore | invalid | *comparer | *transformer
//	Grouping:    Options
type applicableOption interface {
	Option

	// apply executes the option, which may mutate s or panic.
	apply(s *state, vx, vy reflect.Value)
}

// coreOption represents the following types:
//	Fundamental: ignore | invalid | *comparer | *transformer
//	Filters:     *pathFilter | *valuesFilter
type coreOption interface {
	Option
	isCore()
}

type core struct{}

func (core) isCore() {}

// Options is a list of Option values that also satisfies the Option interface.
// Helper comparison packages may return an Options value when packing multiple
// Option values into a single Option. When this package processes an Options,
// it will be implicitly expanded into a flat list.
//
// Applying a filter on an Options is equivalent to applying that same filter
// on all individual options held within.
type Options []Option

func (opts Options) filter(s *state, vx, vy reflect.Value, t reflect.Type) (out applicableOption) {
	for _, opt := range opts {
		switch opt := opt.filter(s, vx, vy, t); opt.(type) {
		case ignore:
			return ignore{} // Only ignore can short-circuit evaluation
		case invalid:
			out = invalid{} // Takes precedence over comparer or transformer
		case *comparer, *transformer, Options:
			switch out.(type) {
			case nil:
				out = opt
			case invalid:
				// Keep invalid
			case *comparer, *transformer, Options:
				out = Options{out, opt} // Conflicting comparers or transformers
			}
		}
	}
	return out
}

func (opts Options) apply(s *state, _, _ reflect.Value) {
	const warning = "ambiguous set of applicable options"
	const help = "consider using filters to ensure at most one Comparer or Transformer may apply"
	var ss []string
	for _, opt := range flattenOptions(nil, opts) {
		ss = append(ss, fmt.Sprint(opt))
	}
	set := strings.Join(ss, "\n\t")
	panic(fmt.Sprintf("%s at %#v:\n\t%s\n%s", warning, s.curPath, set, help))
}

func (opts Options) String() string {
	var ss []string
	for _, opt := range opts {
		ss = append(ss, fmt.Sprint(opt))
	}
	return fmt.Sprintf("Options{%s}", strings.Join(ss, ", "))
}

// FilterPath returns a new Option where opt is only evaluated if filter f
// returns true for the current Path in the value tree.
//
// The option passed in may be an Ignore, Transformer, Comparer, Options, or
// a previously filtered Option.
func FilterPath(f func(Path) bool, opt Option) Option {
	if f == nil {
		panic("invalid path filter function")
	}
	if opt := normalizeOption(opt); opt != nil {
		return &pathFilter{fnc: f, opt: opt}
	}
	return nil
}

type pathFilter struct {
	core
	fnc func(Path) bool
	opt Option
}

func (f pathFilter) filter(s *state, vx, vy reflect.Value, t reflect.Type) applicableOption {
	if f.fnc(s.curPath) {
		return f.opt.filter(s, vx, vy, t)
	}
	return nil
}

func (f pathFilter) String() string {
	fn := getFuncName(reflect.ValueOf(f.fnc).Pointer())
	return fmt.Sprintf("FilterPath(%s, %v)", fn, f.opt)
}

// FilterValues returns a new Option where opt is only evaluated if filter f,
// which is a function of the form "func(T, T) bool", returns true for the
// current pair of values being compared. If the type of the values is not
// assignable to T, then this filter implicitly returns false.
//
// The filter function must be
// symmetric (i.e., agnostic to the order of the inputs) and
// deterministic (i.e., produces the same result when given the same inputs).
// If T is an interface, it is possible that f is called with two values with
// different concrete types that both implement T.
//
// The option passed in may be an Ignore, Transformer, Comparer, Options, or
// a previously filtered Option.
func FilterValues(f interface{}, opt Option) Option {
	v := reflect.ValueOf(f)
	if !function.IsType(v.Type(), function.ValueFilter) || v.IsNil() {
		panic(fmt.Sprintf("invalid values filter function: %T", f))
	}
	if opt := normalizeOption(opt); opt != nil {
		vf := &valuesFilter{fnc: v, opt: opt}
		if ti := v.Type().In(0); ti.Kind() != reflect.Interface || ti.NumMethod() > 0 {
			vf.typ = ti
		}
		return vf
	}
	return nil
}

type valuesFilter struct {
	core
	typ reflect.Type  // T
	fnc reflect.Value // func(T, T) bool
	opt Option
}

func (f valuesFilter) filter(s *state, vx, vy reflect.Value, t reflect.Type) applicableOption {
	if !vx.IsValid() || !vy.IsValid() {
		return invalid{}
	}
	if (f.typ == nil || t.AssignableTo(f.typ)) && s.callTTBFunc(f.fnc, vx, vy) {
		return f.opt.filter(s, vx, vy, t)
	}
	return nil
}

func (f valuesFilter) String() string {
	fn := getFuncName(f.fnc.Pointer())
	return fmt.Sprintf("FilterValues(%s, %v)", fn, f.opt)
}

// Ignore is an Option that causes all comparisons to be ignored.
// This value is intended to be combined with FilterPath or FilterValues.
// It is an error to pass an unfiltered Ignore option to Equal.
func Ignore() Option { return ignore{} }

type ignore struct{ core }

func (ignore) isFiltered() bool                                                     { return false }
func (ignore) filter(_ *state, _, _ reflect.Value, _ reflect.Type) applicableOption { return ignore{} }
func (ignore) apply(_ *state, _, _ reflect.Value)                                   { return }
func (ignore) String() string                                                       { return "Ignore()" }

// invalid is a sentinel Option type to indicate that some options could not
// be evaluated due to unexported fields.
type invalid struct{ core }

func (invalid) filter(_ *state, _, _ reflect.Value, _ reflect.Type) applicableOption { return invalid{} }
func (invalid) apply(s *state, _, _ reflect.Value) {
	const help = "consider using AllowUnexported or cmpopts.IgnoreUnexported"
	panic(fmt.Sprintf("cannot handle unexported field: %#v\n%s", s.curPath, help))
}

// Transformer returns an Option that applies a transformation function that
// converts values of a certain type into that of another.
//
// The transformer f must be a function "func(T) R" that converts values of
// type T to those of type R and is implicitly filtered to input values
// assignable to T. The transformer must not mutate T in any way.
//
// To help prevent some cases of infinite recursive cycles applying the
// same transform to the output of itself (e.g., in the case where the
// input and output types are the same), an implicit filter is added such that
// a transformer is applicable only if that exact transformer is not already
// in the tail of the Path since the last non-Transform step.
//
// The name is a user provided label that is used as the Transform.Name in the
// transformation PathStep. If empty, an arbitrary name is used.
func Transformer(name string, f interface{}) Option {
	v := reflect.ValueOf(f)
	if !function.IsType(v.Type(), function.Transformer) || v.IsNil() {
		panic(fmt.Sprintf("invalid transformer function: %T", f))
	}
	if name == "" {
		name = "λ" // Lambda-symbol as place-holder for anonymous transformer
	}
	if !isValid(name) {
		panic(fmt.Sprintf("invalid name: %q", name))
	}
	tr := &transformer{name: name, fnc: reflect.ValueOf(f)}
	if ti := v.Type().In(0); ti.Kind() != reflect.Interface || ti.NumMethod() > 0 {
		tr.typ = ti
	}
	return tr
}

type transformer struct {
	core
	name string
	typ  reflect.Type  // T
	fnc  reflect.Value // func(T) R
}

func (tr *transformer) isFiltered() bool { return tr.typ != nil }

func (tr *transformer) filter(s *state, _, _ reflect.Value, t reflect.Type) applicableOption {
	for i := len(s.curPath) - 1; i >= 0; i-- {
		if t, ok := s.curPath[i].(*transform); !ok {
			break // Hit most recent non-Transform step
		} else if tr == t.trans {
			return nil // Cannot directly use same Transform
		}
	}
	if tr.typ == nil || t.AssignableTo(tr.typ) {
		return tr
	}
	return nil
}

func (tr *transformer) apply(s *state, vx, vy reflect.Value) {
	// Update path before calling the Transformer so that dynamic checks
	// will use the updated path.
	s.curPath.push(&transform{pathStep{tr.fnc.Type().Out(0)}, tr})
	defer s.curPath.pop()

	vx = s.callTRFunc(tr.fnc, vx)
	vy = s.callTRFunc(tr.fnc, vy)
	s.compareAny(vx, vy)
}

func (tr transformer) String() string {
	return fmt.Sprintf("Transformer(%s, %s)", tr.name, getFuncName(tr.fnc.Pointer()))
}

// Comparer returns an Option that determines whether two values are equal
// to each other.
//
// The comparer f must be a function "func(T, T) bool" and is implicitly
// filtered to input values assignable to T. If T is an interface, it is
// possible that f is called with two values of different concrete types that
// both implement T.
//
// The equality function must be:
//	• Symmetric: equal(x, y) == equal(y, x)
//	• Deterministic: equal(x, y) == equal(x, y)
//	• Pure: equal(x, y) does not modify x or y
func Comparer(f interface{}) Option {
	v := reflect.ValueOf(f)
	if !function.IsType(v.Type(), function.Equal) || v.IsNil() {
		panic(fmt.Sprintf("invalid comparer function: %T", f))
	}
	cm := &comparer{fnc: v}
	if ti := v.Type().In(0); ti.Kind() != reflect.Interface || ti.NumMethod() > 0 {
		cm.typ = ti
	}
	return cm
}

type comparer struct {
	core
	typ reflect.Type  // T
	fnc reflect.Value // func(T, T) bool
}

func (cm *comparer) isFiltered() bool { return cm.typ != nil }

func (cm *comparer) filter(_ *state, _, _ reflect.Value, t reflect.Type) applicableOption {
	if cm.typ == nil || t.AssignableTo(cm.typ) {
		return cm
	}
	return nil
}

func (cm *comparer) apply(s *state, vx, vy reflect.Value) {
	eq := s.callTTBFunc(cm.fnc, vx, vy)
	s.report(eq, vx, vy)
}

func (cm comparer) String() string {
	return fmt.Sprintf("Comparer(%s)", getFuncName(cm.fnc.Pointer()))
}

// AllowUnexported returns an Option that forcibly allows operations on
// unexported fields in certain structs, which are specified by passing in a
// value of each struct type.
//
// Users of this option must understand that comparing on unexported fields
// from external packages is not safe since changes in the internal
// implementation of some external package may cause the result of Equal
// to unexpectedly change. However, it may be valid to use this option on types
// defined in an internal package where the semantic meaning of an unexported
// field is in the control of the user.
//
// For some cases, a custom Comparer should be used instead that defines
// equality as a function of the public API of a type rather than the underlying
// unexported implementation.
//
// For example, the reflect.Type documentation defines equality to be determined
// by the == operator on the interface (essentially performing a shallow pointer
// comparison) and most attempts to compare *regexp.Regexp types are interested
// in only checking that the regular expression strings are equal.
// Both of these are accomplished using Comparers:
//
//	Comparer(func(x, y reflect.Type) bool { return x == y })
//	Comparer(func(x, y *regexp.Regexp) bool { return x.String() == y.String() })
//
// In other cases, the cmpopts.IgnoreUnexported option can be used to ignore
// all unexported fields on specified struct types.
func AllowUnexported(types ...interface{}) Option {
	if !supportAllowUnexported {
		panic("AllowUnexported is not supported on purego builds, Google App Engine Standard, or GopherJS")
	}
	m := make(map[reflect.Type]bool)
	for _, typ := range types {
		t := reflect.TypeOf(typ)
		if t.Kind() != reflect.Struct {
			panic(fmt.Sprintf("invalid struct type: %T", typ))
		}
		m[t] = true
	}
	return visibleStructs(m)
}

type visibleStructs map[reflect.Type]bool

func (visibleStructs) filter(_ *state, _, _ reflect.Value, _ reflect.Type) applicableOption {
	panic("not implemented")
}

// reporter is an Option that configures how differences are reported.
type reporter interface {
	// TODO: Not exported yet.
	//
	// Perhaps add PushStep and PopStep and change Report to only accept
	// a PathStep instead of the full-path? Adding a PushStep and PopStep makes
	// it clear that we are traversing the value tree in a depth-first-search
	// manner, which has an effect on how values are printed.

	Option

	// Report is called for every comparison made and will be provided with
	// the two values being compared, the equality result, and the
	// current path in the value tree. It is possible for x or y to be an
	// invalid reflect.Value if one of the values is non-existent;
	// which is possible with maps and slices.
	Report(x, y reflect.Value, eq bool, p Path)
}

// normalizeOption normalizes the input options such that all Options groups
// are flattened and groups with a single element are reduced to that element.
// Only coreOptions and Options containing coreOptions are allowed.
func normalizeOption(src Option) Option {
	switch opts := flattenOptions(nil, Options{src}); len(opts) {
	case 0:
		return nil
	case 1:
		return opts[0]
	default:
		return opts
	}
}

// flattenOptions copies all options in src to dst as a flat list.
// Only coreOptions and Options containing coreOptions are allowed.
func flattenOptions(dst, src Options) Options {
	for _, opt := range src {
		switch opt := opt.(type) {
		case nil:
			continue
		case Options:
			dst = flattenOptions(dst, opt)
		case coreOption:
			dst = append(dst, opt)
		default:
			panic(fmt.Sprintf("invalid option type: %T", opt))
		}
	}
	return dst
}

// getFuncName returns a short function name from the pointer.
// The string parsing logic works up until Go1.9.
func getFuncName(p uintptr) string {
	fnc := runtime.FuncForPC(p)
	if fnc == nil {
		return "<unknown>"
	}
	name := fnc.Name() // E.g., "long/path/name/mypkg.(mytype).(long/path/name/mypkg.myfunc)-fm"
	if strings.HasSuffix(name, ")-fm") || strings.HasSuffix(name, ")·fm") {
		// Strip the package name from method name.
		name = strings.TrimSuffix(name, ")-fm")
		name = strings.TrimSuffix(name, ")·fm")
		if i := strings.LastIndexByte(name, '('); i >= 0 {
			methodName := name[i+1:] // E.g., "long/path/name/mypkg.myfunc"
			if j := strings.LastIndexByte(methodName, '.'); j >= 0 {
				methodName = methodName[j+1:] // E.g., "myfunc"
			}
			name = name[:i] + methodName // E.g., "long/path/name/mypkg.(mytype)." + "myfunc"
		}
	}
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		// Strip the package name.
		name = name[i+1:] // E.g., "mypkg.(mytype).myfunc"
	}
	return name
}
