// Copyright 2017, The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package value provides functionality for reflect.Value types.
package value

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

var stringerIface = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()

// Format formats the value v as a string.
//
// This is similar to fmt.Sprintf("%+v", v) except this:
//	* Prints the type unless it can be elided
//	* Avoids printing struct fields that are zero
//	* Prints a nil-slice as being nil, not empty
//	* Prints map entries in deterministic order
func Format(v reflect.Value, conf FormatConfig) string {
	conf.printType = true
	conf.followPointers = true
	conf.realPointers = true
	return formatAny(v, conf, nil)
}

type FormatConfig struct {
	UseStringer        bool // Should the String method be used if available?
	printType          bool // Should we print the type before the value?
	PrintPrimitiveType bool // Should we print the type of primitives?
	followPointers     bool // Should we recursively follow pointers?
	realPointers       bool // Should we print the real address of pointers?
}

func formatAny(v reflect.Value, conf FormatConfig, visited map[uintptr]bool) string {
	// TODO: Should this be a multi-line printout in certain situations?

	if !v.IsValid() {
		return "<non-existent>"
	}
	if conf.UseStringer && v.Type().Implements(stringerIface) && v.CanInterface() {
		if (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) && v.IsNil() {
			return "<nil>"
		}

		const stringerPrefix = "s" // Indicates that the String method was used
		s := v.Interface().(fmt.Stringer).String()
		return stringerPrefix + formatString(s)
	}

	switch v.Kind() {
	case reflect.Bool:
		return formatPrimitive(v.Type(), v.Bool(), conf)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return formatPrimitive(v.Type(), v.Int(), conf)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if v.Type().PkgPath() == "" || v.Kind() == reflect.Uintptr {
			// Unnamed uints are usually bytes or words, so use hexadecimal.
			return formatPrimitive(v.Type(), formatHex(v.Uint()), conf)
		}
		return formatPrimitive(v.Type(), v.Uint(), conf)
	case reflect.Float32, reflect.Float64:
		return formatPrimitive(v.Type(), v.Float(), conf)
	case reflect.Complex64, reflect.Complex128:
		return formatPrimitive(v.Type(), v.Complex(), conf)
	case reflect.String:
		return formatPrimitive(v.Type(), formatString(v.String()), conf)
	case reflect.UnsafePointer, reflect.Chan, reflect.Func:
		return formatPointer(v, conf)
	case reflect.Ptr:
		if v.IsNil() {
			if conf.printType {
				return fmt.Sprintf("(%v)(nil)", v.Type())
			}
			return "<nil>"
		}
		if visited[v.Pointer()] || !conf.followPointers {
			return formatPointer(v, conf)
		}
		visited = insertPointer(visited, v.Pointer())
		return "&" + formatAny(v.Elem(), conf, visited)
	case reflect.Interface:
		if v.IsNil() {
			if conf.printType {
				return fmt.Sprintf("%v(nil)", v.Type())
			}
			return "<nil>"
		}
		return formatAny(v.Elem(), conf, visited)
	case reflect.Slice:
		if v.IsNil() {
			if conf.printType {
				return fmt.Sprintf("%v(nil)", v.Type())
			}
			return "<nil>"
		}
		if visited[v.Pointer()] {
			return formatPointer(v, conf)
		}
		visited = insertPointer(visited, v.Pointer())
		fallthrough
	case reflect.Array:
		var ss []string
		subConf := conf
		subConf.printType = v.Type().Elem().Kind() == reflect.Interface
		for i := 0; i < v.Len(); i++ {
			s := formatAny(v.Index(i), subConf, visited)
			ss = append(ss, s)
		}
		s := fmt.Sprintf("{%s}", strings.Join(ss, ", "))
		if conf.printType {
			return v.Type().String() + s
		}
		return s
	case reflect.Map:
		if v.IsNil() {
			if conf.printType {
				return fmt.Sprintf("%v(nil)", v.Type())
			}
			return "<nil>"
		}
		if visited[v.Pointer()] {
			return formatPointer(v, conf)
		}
		visited = insertPointer(visited, v.Pointer())

		var ss []string
		keyConf, valConf := conf, conf
		keyConf.printType = v.Type().Key().Kind() == reflect.Interface
		keyConf.followPointers = false
		valConf.printType = v.Type().Elem().Kind() == reflect.Interface
		for _, k := range SortKeys(v.MapKeys()) {
			sk := formatAny(k, keyConf, visited)
			sv := formatAny(v.MapIndex(k), valConf, visited)
			ss = append(ss, fmt.Sprintf("%s: %s", sk, sv))
		}
		s := fmt.Sprintf("{%s}", strings.Join(ss, ", "))
		if conf.printType {
			return v.Type().String() + s
		}
		return s
	case reflect.Struct:
		var ss []string
		subConf := conf
		subConf.printType = true
		for i := 0; i < v.NumField(); i++ {
			vv := v.Field(i)
			if isZero(vv) {
				continue // Elide zero value fields
			}
			name := v.Type().Field(i).Name
			subConf.UseStringer = conf.UseStringer
			s := formatAny(vv, subConf, visited)
			ss = append(ss, fmt.Sprintf("%s: %s", name, s))
		}
		s := fmt.Sprintf("{%s}", strings.Join(ss, ", "))
		if conf.printType {
			return v.Type().String() + s
		}
		return s
	default:
		panic(fmt.Sprintf("%v kind not handled", v.Kind()))
	}
}

func formatString(s string) string {
	// Use quoted string if it the same length as a raw string literal.
	// Otherwise, attempt to use the raw string form.
	qs := strconv.Quote(s)
	if len(qs) == 1+len(s)+1 {
		return qs
	}

	// Disallow newlines to ensure output is a single line.
	// Only allow printable runes for readability purposes.
	rawInvalid := func(r rune) bool {
		return r == '`' || r == '\n' || !unicode.IsPrint(r)
	}
	if strings.IndexFunc(s, rawInvalid) < 0 {
		return "`" + s + "`"
	}
	return qs
}

func formatPrimitive(t reflect.Type, v interface{}, conf FormatConfig) string {
	if conf.printType && (conf.PrintPrimitiveType || t.PkgPath() != "") {
		return fmt.Sprintf("%v(%v)", t, v)
	}
	return fmt.Sprintf("%v", v)
}

func formatPointer(v reflect.Value, conf FormatConfig) string {
	p := v.Pointer()
	if !conf.realPointers {
		p = 0 // For deterministic printing purposes
	}
	s := formatHex(uint64(p))
	if conf.printType {
		return fmt.Sprintf("(%v)(%s)", v.Type(), s)
	}
	return s
}

func formatHex(u uint64) string {
	var f string
	switch {
	case u <= 0xff:
		f = "0x%02x"
	case u <= 0xffff:
		f = "0x%04x"
	case u <= 0xffffff:
		f = "0x%06x"
	case u <= 0xffffffff:
		f = "0x%08x"
	case u <= 0xffffffffff:
		f = "0x%010x"
	case u <= 0xffffffffffff:
		f = "0x%012x"
	case u <= 0xffffffffffffff:
		f = "0x%014x"
	case u <= 0xffffffffffffffff:
		f = "0x%016x"
	}
	return fmt.Sprintf(f, u)
}

// insertPointer insert p into m, allocating m if necessary.
func insertPointer(m map[uintptr]bool, p uintptr) map[uintptr]bool {
	if m == nil {
		m = make(map[uintptr]bool)
	}
	m[p] = true
	return m
}

// isZero reports whether v is the zero value.
// This does not rely on Interface and so can be used on unexported fields.
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool() == false
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.UnsafePointer:
		return v.Pointer() == 0
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice:
		return v.IsNil()
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !isZero(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	}
	return false
}
