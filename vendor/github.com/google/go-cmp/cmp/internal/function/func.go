// Copyright 2017, The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package function identifies function types.
package function

import "reflect"

type funcType int

const (
	_ funcType = iota

	ttbFunc // func(T, T) bool
	tibFunc // func(T, I) bool
	trFunc  // func(T) R

	Equal           = ttbFunc // func(T, T) bool
	EqualAssignable = tibFunc // func(T, I) bool; encapsulates func(T, T) bool
	Transformer     = trFunc  // func(T) R
	ValueFilter     = ttbFunc // func(T, T) bool
	Less            = ttbFunc // func(T, T) bool
)

var boolType = reflect.TypeOf(true)

// IsType reports whether the reflect.Type is of the specified function type.
func IsType(t reflect.Type, ft funcType) bool {
	if t == nil || t.Kind() != reflect.Func || t.IsVariadic() {
		return false
	}
	ni, no := t.NumIn(), t.NumOut()
	switch ft {
	case ttbFunc: // func(T, T) bool
		if ni == 2 && no == 1 && t.In(0) == t.In(1) && t.Out(0) == boolType {
			return true
		}
	case tibFunc: // func(T, I) bool
		if ni == 2 && no == 1 && t.In(0).AssignableTo(t.In(1)) && t.Out(0) == boolType {
			return true
		}
	case trFunc: // func(T) R
		if ni == 1 && no == 1 {
			return true
		}
	}
	return false
}
