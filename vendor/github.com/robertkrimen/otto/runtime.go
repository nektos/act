package otto

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/robertkrimen/otto/ast"
	"github.com/robertkrimen/otto/parser"
)

type _global struct {
	Object         *_object // Object( ... ), new Object( ... ) - 1 (length)
	Function       *_object // Function( ... ), new Function( ... ) - 1
	Array          *_object // Array( ... ), new Array( ... ) - 1
	String         *_object // String( ... ), new String( ... ) - 1
	Boolean        *_object // Boolean( ... ), new Boolean( ... ) - 1
	Number         *_object // Number( ... ), new Number( ... ) - 1
	Math           *_object
	Date           *_object // Date( ... ), new Date( ... ) - 7
	RegExp         *_object // RegExp( ... ), new RegExp( ... ) - 2
	Error          *_object // Error( ... ), new Error( ... ) - 1
	EvalError      *_object
	TypeError      *_object
	RangeError     *_object
	ReferenceError *_object
	SyntaxError    *_object
	URIError       *_object
	JSON           *_object

	ObjectPrototype         *_object // Object.prototype
	FunctionPrototype       *_object // Function.prototype
	ArrayPrototype          *_object // Array.prototype
	StringPrototype         *_object // String.prototype
	BooleanPrototype        *_object // Boolean.prototype
	NumberPrototype         *_object // Number.prototype
	DatePrototype           *_object // Date.prototype
	RegExpPrototype         *_object // RegExp.prototype
	ErrorPrototype          *_object // Error.prototype
	EvalErrorPrototype      *_object
	TypeErrorPrototype      *_object
	RangeErrorPrototype     *_object
	ReferenceErrorPrototype *_object
	SyntaxErrorPrototype    *_object
	URIErrorPrototype       *_object
}

type _runtime struct {
	global       _global
	globalObject *_object
	globalStash  *_objectStash
	scope        *_scope
	otto         *Otto
	eval         *_object // The builtin eval, for determine indirect versus direct invocation
	debugger     func(*Otto)
	random       func() float64
	stackLimit   int
	traceLimit   int

	labels []string // FIXME
	lck    sync.Mutex
}

func (self *_runtime) enterScope(scope *_scope) {
	scope.outer = self.scope
	if self.scope != nil {
		if self.stackLimit != 0 && self.scope.depth+1 >= self.stackLimit {
			panic(self.panicRangeError("Maximum call stack size exceeded"))
		}

		scope.depth = self.scope.depth + 1
	}

	self.scope = scope
}

func (self *_runtime) leaveScope() {
	self.scope = self.scope.outer
}

// FIXME This is used in two places (cloning)
func (self *_runtime) enterGlobalScope() {
	self.enterScope(newScope(self.globalStash, self.globalStash, self.globalObject))
}

func (self *_runtime) enterFunctionScope(outer _stash, this Value) *_fnStash {
	if outer == nil {
		outer = self.globalStash
	}
	stash := self.newFunctionStash(outer)
	var thisObject *_object
	switch this.kind {
	case valueUndefined, valueNull:
		thisObject = self.globalObject
	default:
		thisObject = self.toObject(this)
	}
	self.enterScope(newScope(stash, stash, thisObject))
	return stash
}

func (self *_runtime) putValue(reference _reference, value Value) {
	name := reference.putValue(value)
	if name != "" {
		// Why? -- If reference.base == nil
		// strict = false
		self.globalObject.defineProperty(name, value, 0111, false)
	}
}

func (self *_runtime) tryCatchEvaluate(inner func() Value) (tryValue Value, exception bool) {
	// resultValue = The value of the block (e.g. the last statement)
	// throw = Something was thrown
	// throwValue = The value of what was thrown
	// other = Something that changes flow (return, break, continue) that is not a throw
	// Otherwise, some sort of unknown panic happened, we'll just propagate it
	defer func() {
		if caught := recover(); caught != nil {
			if exception, ok := caught.(*_exception); ok {
				caught = exception.eject()
			}
			switch caught := caught.(type) {
			case _error:
				exception = true
				tryValue = toValue_object(self.newError(caught.name, caught.messageValue(), 0))
			case Value:
				exception = true
				tryValue = caught
			default:
				panic(caught)
			}
		}
	}()

	tryValue = inner()
	return
}

// toObject

func (self *_runtime) toObject(value Value) *_object {
	switch value.kind {
	case valueEmpty, valueUndefined, valueNull:
		panic(self.panicTypeError())
	case valueBoolean:
		return self.newBoolean(value)
	case valueString:
		return self.newString(value)
	case valueNumber:
		return self.newNumber(value)
	case valueObject:
		return value._object()
	}
	panic(self.panicTypeError())
}

func (self *_runtime) objectCoerce(value Value) (*_object, error) {
	switch value.kind {
	case valueUndefined:
		return nil, errors.New("undefined")
	case valueNull:
		return nil, errors.New("null")
	case valueBoolean:
		return self.newBoolean(value), nil
	case valueString:
		return self.newString(value), nil
	case valueNumber:
		return self.newNumber(value), nil
	case valueObject:
		return value._object(), nil
	}
	panic(self.panicTypeError())
}

func checkObjectCoercible(rt *_runtime, value Value) {
	isObject, mustCoerce := testObjectCoercible(value)
	if !isObject && !mustCoerce {
		panic(rt.panicTypeError())
	}
}

// testObjectCoercible

func testObjectCoercible(value Value) (isObject bool, mustCoerce bool) {
	switch value.kind {
	case valueReference, valueEmpty, valueNull, valueUndefined:
		return false, false
	case valueNumber, valueString, valueBoolean:
		return false, true
	case valueObject:
		return true, false
	default:
		panic("this should never happen")
	}
}

func (self *_runtime) safeToValue(value interface{}) (Value, error) {
	result := Value{}
	err := catchPanic(func() {
		result = self.toValue(value)
	})
	return result, err
}

// convertNumeric converts numeric parameter val from js to that of type t if it is safe to do so, otherwise it panics.
// This allows literals (int64), bitwise values (int32) and the general form (float64) of javascript numerics to be passed as parameters to go functions easily.
func (self *_runtime) convertNumeric(v Value, t reflect.Type) reflect.Value {
	val := reflect.ValueOf(v.export())

	if val.Kind() == t.Kind() {
		return val
	}

	if val.Kind() == reflect.Interface {
		val = reflect.ValueOf(val.Interface())
	}

	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		f64 := val.Float()
		switch t.Kind() {
		case reflect.Float64:
			return reflect.ValueOf(f64)
		case reflect.Float32:
			if reflect.Zero(t).OverflowFloat(f64) {
				panic(self.panicRangeError("converting float64 to float32 would overflow"))
			}

			return val.Convert(t)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			i64 := int64(f64)
			if float64(i64) != f64 {
				panic(self.panicRangeError(fmt.Sprintf("converting %v to %v would cause loss of precision", val.Type(), t)))
			}

			// The float represents an integer
			val = reflect.ValueOf(i64)
		default:
			panic(self.panicTypeError(fmt.Sprintf("cannot convert %v to %v", val.Type(), t)))
		}
	}

	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i64 := val.Int()
		switch t.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if reflect.Zero(t).OverflowInt(i64) {
				panic(self.panicRangeError(fmt.Sprintf("converting %v to %v would overflow", val.Type(), t)))
			}
			return val.Convert(t)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if i64 < 0 {
				panic(self.panicRangeError(fmt.Sprintf("converting %v to %v would underflow", val.Type(), t)))
			}
			if reflect.Zero(t).OverflowUint(uint64(i64)) {
				panic(self.panicRangeError(fmt.Sprintf("converting %v to %v would overflow", val.Type(), t)))
			}
			return val.Convert(t)
		case reflect.Float32, reflect.Float64:
			return val.Convert(t)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u64 := val.Uint()
		switch t.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if u64 > math.MaxInt64 || reflect.Zero(t).OverflowInt(int64(u64)) {
				panic(self.panicRangeError(fmt.Sprintf("converting %v to %v would overflow", val.Type(), t)))
			}
			return val.Convert(t)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if reflect.Zero(t).OverflowUint(u64) {
				panic(self.panicRangeError(fmt.Sprintf("converting %v to %v would overflow", val.Type(), t)))
			}
			return val.Convert(t)
		case reflect.Float32, reflect.Float64:
			return val.Convert(t)
		}
	}

	panic(self.panicTypeError(fmt.Sprintf("unsupported type %v -> %v for numeric conversion", val.Type(), t)))
}

func fieldIndexByName(t reflect.Type, name string) []int {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		if !validGoStructName(f.Name) {
			continue
		}

		if f.Anonymous {
			if a := fieldIndexByName(f.Type, name); a != nil {
				return append([]int{i}, a...)
			}
		}

		if a := strings.SplitN(f.Tag.Get("json"), ",", 2); a[0] != "" {
			if a[0] == "-" {
				continue
			}

			if a[0] == name {
				return []int{i}
			}
		}

		if f.Name == name {
			return []int{i}
		}
	}

	return nil
}

var typeOfValue = reflect.TypeOf(Value{})
var typeOfJSONRawMessage = reflect.TypeOf(json.RawMessage{})

// convertCallParameter converts request val to type t if possible.
// If the conversion fails due to overflow or type miss-match then it panics.
// If no conversion is known then the original value is returned.
func (self *_runtime) convertCallParameter(v Value, t reflect.Type) reflect.Value {
	if t == typeOfValue {
		return reflect.ValueOf(v)
	}

	if t == typeOfJSONRawMessage {
		if d, err := json.Marshal(v.export()); err == nil {
			return reflect.ValueOf(d)
		}
	}

	if v.kind == valueObject {
		if gso, ok := v._object().value.(*_goStructObject); ok {
			if gso.value.Type().AssignableTo(t) {
				// please see TestDynamicFunctionReturningInterface for why this exists
				if t.Kind() == reflect.Interface && gso.value.Type().ConvertibleTo(t) {
					return gso.value.Convert(t)
				} else {
					return gso.value
				}
			}
		}

		if gao, ok := v._object().value.(*_goArrayObject); ok {
			if gao.value.Type().AssignableTo(t) {
				// please see TestDynamicFunctionReturningInterface for why this exists
				if t.Kind() == reflect.Interface && gao.value.Type().ConvertibleTo(t) {
					return gao.value.Convert(t)
				} else {
					return gao.value
				}
			}
		}
	}

	if t.Kind() == reflect.Interface {
		e := v.export()
		if e == nil {
			return reflect.Zero(t)
		}
		iv := reflect.ValueOf(e)
		if iv.Type().AssignableTo(t) {
			return iv
		}
	}

	tk := t.Kind()

	if tk == reflect.Ptr {
		switch v.kind {
		case valueEmpty, valueNull, valueUndefined:
			return reflect.Zero(t)
		default:
			var vv reflect.Value
			if err := catchPanic(func() { vv = self.convertCallParameter(v, t.Elem()) }); err == nil {
				if vv.CanAddr() {
					return vv.Addr()
				}

				pv := reflect.New(vv.Type())
				pv.Elem().Set(vv)
				return pv
			}
		}
	}

	switch tk {
	case reflect.Bool:
		return reflect.ValueOf(v.bool())
	case reflect.String:
		switch v.kind {
		case valueString:
			return reflect.ValueOf(v.value)
		case valueNumber:
			return reflect.ValueOf(fmt.Sprintf("%v", v.value))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		switch v.kind {
		case valueNumber:
			return self.convertNumeric(v, t)
		}
	case reflect.Slice:
		if o := v._object(); o != nil {
			if lv := o.get("length"); lv.IsNumber() {
				l := lv.number().int64

				s := reflect.MakeSlice(t, int(l), int(l))

				tt := t.Elem()

				if o.class == "Array" {
					for i := int64(0); i < l; i++ {
						p, ok := o.property[strconv.FormatInt(i, 10)]
						if !ok {
							continue
						}

						e, ok := p.value.(Value)
						if !ok {
							continue
						}

						ev := self.convertCallParameter(e, tt)

						s.Index(int(i)).Set(ev)
					}
				} else if o.class == "GoArray" {

					var gslice bool
					switch o.value.(type) {
					case *_goSliceObject:
						gslice = true
					case *_goArrayObject:
						gslice = false
					}

					for i := int64(0); i < l; i++ {
						var p *_property
						if gslice {
							p = goSliceGetOwnProperty(o, strconv.FormatInt(i, 10))
						} else {
							p = goArrayGetOwnProperty(o, strconv.FormatInt(i, 10))
						}
						if p == nil {
							continue
						}

						e, ok := p.value.(Value)
						if !ok {
							continue
						}

						ev := self.convertCallParameter(e, tt)

						s.Index(int(i)).Set(ev)
					}
				}

				return s
			}
		}
	case reflect.Map:
		if o := v._object(); o != nil && t.Key().Kind() == reflect.String {
			m := reflect.MakeMap(t)

			o.enumerate(false, func(k string) bool {
				m.SetMapIndex(reflect.ValueOf(k), self.convertCallParameter(o.get(k), t.Elem()))
				return true
			})

			return m
		}
	case reflect.Func:
		if t.NumOut() > 1 {
			panic(self.panicTypeError("converting JavaScript values to Go functions with more than one return value is currently not supported"))
		}

		if o := v._object(); o != nil && o.class == "Function" {
			return reflect.MakeFunc(t, func(args []reflect.Value) []reflect.Value {
				l := make([]interface{}, len(args))
				for i, a := range args {
					if a.CanInterface() {
						l[i] = a.Interface()
					}
				}

				rv, err := v.Call(nullValue, l...)
				if err != nil {
					panic(err)
				}

				if t.NumOut() == 0 {
					return nil
				}

				return []reflect.Value{self.convertCallParameter(rv, t.Out(0))}
			})
		}
	case reflect.Struct:
		if o := v._object(); o != nil && o.class == "Object" {
			s := reflect.New(t)

			for _, k := range o.propertyOrder {
				idx := fieldIndexByName(t, k)

				if idx == nil {
					panic(self.panicTypeError("can't convert object; field %q was supplied but does not exist on target %v", k, t))
				}

				ss := s

				for _, i := range idx {
					if ss.Kind() == reflect.Ptr {
						if ss.IsNil() {
							if !ss.CanSet() {
								panic(self.panicTypeError("can't set embedded pointer to unexported struct: %v", ss.Type().Elem()))
							}

							ss.Set(reflect.New(ss.Type().Elem()))
						}

						ss = ss.Elem()
					}

					ss = ss.Field(i)
				}

				ss.Set(self.convertCallParameter(o.get(k), ss.Type()))
			}

			return s.Elem()
		}
	}

	if tk == reflect.String {
		if o := v._object(); o != nil && o.hasProperty("toString") {
			if fn := o.get("toString"); fn.IsFunction() {
				sv, err := fn.Call(v)
				if err != nil {
					panic(err)
				}

				var r reflect.Value
				if err := catchPanic(func() { r = self.convertCallParameter(sv, t) }); err == nil {
					return r
				}
			}
		}

		return reflect.ValueOf(v.String())
	}

	if v.kind == valueString {
		var s encoding.TextUnmarshaler

		if reflect.PtrTo(t).Implements(reflect.TypeOf(&s).Elem()) {
			r := reflect.New(t)

			if err := r.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(v.string())); err != nil {
				panic(self.panicSyntaxError("can't convert to %s: %s", t.String(), err.Error()))
			}

			return r.Elem()
		}
	}

	s := "OTTO DOES NOT UNDERSTAND THIS TYPE"
	switch v.kind {
	case valueBoolean:
		s = "boolean"
	case valueNull:
		s = "null"
	case valueNumber:
		s = "number"
	case valueString:
		s = "string"
	case valueUndefined:
		s = "undefined"
	case valueObject:
		s = v.Class()
	}

	panic(self.panicTypeError("can't convert from %q to %q", s, t))
}

func (self *_runtime) toValue(value interface{}) Value {
	switch value := value.(type) {
	case Value:
		return value
	case func(FunctionCall) Value:
		var name, file string
		var line int
		pc := reflect.ValueOf(value).Pointer()
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			name = fn.Name()
			file, line = fn.FileLine(pc)
			file = path.Base(file)
		}
		return toValue_object(self.newNativeFunction(name, file, line, value))
	case _nativeFunction:
		var name, file string
		var line int
		pc := reflect.ValueOf(value).Pointer()
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			name = fn.Name()
			file, line = fn.FileLine(pc)
			file = path.Base(file)
		}
		return toValue_object(self.newNativeFunction(name, file, line, value))
	case Object, *Object, _object, *_object:
		// Nothing happens.
		// FIXME We should really figure out what can come here.
		// This catch-all is ugly.
	default:
		{
			value := reflect.ValueOf(value)

			switch value.Kind() {
			case reflect.Ptr:
				switch reflect.Indirect(value).Kind() {
				case reflect.Struct:
					return toValue_object(self.newGoStructObject(value))
				case reflect.Array:
					return toValue_object(self.newGoArray(value))
				}
			case reflect.Struct:
				return toValue_object(self.newGoStructObject(value))
			case reflect.Map:
				return toValue_object(self.newGoMapObject(value))
			case reflect.Slice:
				return toValue_object(self.newGoSlice(value))
			case reflect.Array:
				return toValue_object(self.newGoArray(value))
			case reflect.Func:
				var name, file string
				var line int
				if v := reflect.ValueOf(value); v.Kind() == reflect.Ptr {
					pc := v.Pointer()
					fn := runtime.FuncForPC(pc)
					if fn != nil {
						name = fn.Name()
						file, line = fn.FileLine(pc)
						file = path.Base(file)
					}
				}

				typ := value.Type()

				return toValue_object(self.newNativeFunction(name, file, line, func(c FunctionCall) Value {
					nargs := typ.NumIn()

					if len(c.ArgumentList) != nargs {
						if typ.IsVariadic() {
							if len(c.ArgumentList) < nargs-1 {
								panic(self.panicRangeError(fmt.Sprintf("expected at least %d arguments; got %d", nargs-1, len(c.ArgumentList))))
							}
						} else {
							panic(self.panicRangeError(fmt.Sprintf("expected %d argument(s); got %d", nargs, len(c.ArgumentList))))
						}
					}

					in := make([]reflect.Value, len(c.ArgumentList))

					callSlice := false

					for i, a := range c.ArgumentList {
						var t reflect.Type

						n := i
						if n >= nargs-1 && typ.IsVariadic() {
							if n > nargs-1 {
								n = nargs - 1
							}

							t = typ.In(n).Elem()
						} else {
							t = typ.In(n)
						}

						// if this is a variadic Go function, and the caller has supplied
						// exactly the number of JavaScript arguments required, and this
						// is the last JavaScript argument, try treating the it as the
						// actual set of variadic Go arguments. if that succeeds, break
						// out of the loop.
						if typ.IsVariadic() && len(c.ArgumentList) == nargs && i == nargs-1 {
							var v reflect.Value
							if err := catchPanic(func() { v = self.convertCallParameter(a, typ.In(n)) }); err == nil {
								in[i] = v
								callSlice = true
								break
							}
						}

						in[i] = self.convertCallParameter(a, t)
					}

					var out []reflect.Value
					if callSlice {
						out = value.CallSlice(in)
					} else {
						out = value.Call(in)
					}

					switch len(out) {
					case 0:
						return Value{}
					case 1:
						return self.toValue(out[0].Interface())
					default:
						s := make([]interface{}, len(out))
						for i, v := range out {
							s[i] = self.toValue(v.Interface())
						}

						return self.toValue(s)
					}
				}))
			}
		}
	}

	return toValue(value)
}

func (runtime *_runtime) newGoSlice(value reflect.Value) *_object {
	self := runtime.newGoSliceObject(value)
	self.prototype = runtime.global.ArrayPrototype
	return self
}

func (runtime *_runtime) newGoArray(value reflect.Value) *_object {
	self := runtime.newGoArrayObject(value)
	self.prototype = runtime.global.ArrayPrototype
	return self
}

func (runtime *_runtime) parse(filename string, src, sm interface{}) (*ast.Program, error) {
	return parser.ParseFileWithSourceMap(nil, filename, src, sm, 0)
}

func (runtime *_runtime) cmpl_parse(filename string, src, sm interface{}) (*_nodeProgram, error) {
	program, err := parser.ParseFileWithSourceMap(nil, filename, src, sm, 0)
	if err != nil {
		return nil, err
	}

	return cmpl_parse(program), nil
}

func (self *_runtime) parseSource(src, sm interface{}) (*_nodeProgram, *ast.Program, error) {
	switch src := src.(type) {
	case *ast.Program:
		return nil, src, nil
	case *Script:
		return src.program, nil, nil
	}

	program, err := self.parse("", src, sm)

	return nil, program, err
}

func (self *_runtime) cmpl_runOrEval(src, sm interface{}, eval bool) (Value, error) {
	result := Value{}
	cmpl_program, program, err := self.parseSource(src, sm)
	if err != nil {
		return result, err
	}
	if cmpl_program == nil {
		cmpl_program = cmpl_parse(program)
	}
	err = catchPanic(func() {
		result = self.cmpl_evaluate_nodeProgram(cmpl_program, eval)
	})
	switch result.kind {
	case valueEmpty:
		result = Value{}
	case valueReference:
		result = result.resolve()
	}
	return result, err
}

func (self *_runtime) cmpl_run(src, sm interface{}) (Value, error) {
	return self.cmpl_runOrEval(src, sm, false)
}

func (self *_runtime) cmpl_eval(src, sm interface{}) (Value, error) {
	return self.cmpl_runOrEval(src, sm, true)
}

func (self *_runtime) parseThrow(err error) {
	if err == nil {
		return
	}
	switch err := err.(type) {
	case parser.ErrorList:
		{
			err := err[0]
			if err.Message == "Invalid left-hand side in assignment" {
				panic(self.panicReferenceError(err.Message))
			}
			panic(self.panicSyntaxError(err.Message))
		}
	}
	panic(self.panicSyntaxError(err.Error()))
}

func (self *_runtime) cmpl_parseOrThrow(src, sm interface{}) *_nodeProgram {
	program, err := self.cmpl_parse("", src, sm)
	self.parseThrow(err) // Will panic/throw appropriately
	return program
}
