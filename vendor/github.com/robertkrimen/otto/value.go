package otto

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"unicode/utf16"
)

type _valueKind int

const (
	valueUndefined _valueKind = iota
	valueNull
	valueNumber
	valueString
	valueBoolean
	valueObject

	// These are invalid outside of the runtime
	valueEmpty
	valueResult
	valueReference
)

// Value is the representation of a JavaScript value.
type Value struct {
	kind  _valueKind
	value interface{}
}

func (value Value) safe() bool {
	return value.kind < valueEmpty
}

var (
	emptyValue = Value{kind: valueEmpty}
	nullValue  = Value{kind: valueNull}
	falseValue = Value{kind: valueBoolean, value: false}
	trueValue  = Value{kind: valueBoolean, value: true}
)

// ToValue will convert an interface{} value to a value digestible by otto/JavaScript
//
// This function will not work for advanced types (struct, map, slice/array, etc.) and
// you should use Otto.ToValue instead.
func ToValue(value interface{}) (Value, error) {
	result := Value{}
	err := catchPanic(func() {
		result = toValue(value)
	})
	return result, err
}

func (value Value) isEmpty() bool {
	return value.kind == valueEmpty
}

// Undefined

// UndefinedValue will return a Value representing undefined.
func UndefinedValue() Value {
	return Value{}
}

// IsDefined will return false if the value is undefined, and true otherwise.
func (value Value) IsDefined() bool {
	return value.kind != valueUndefined
}

// IsUndefined will return true if the value is undefined, and false otherwise.
func (value Value) IsUndefined() bool {
	return value.kind == valueUndefined
}

// NullValue will return a Value representing null.
func NullValue() Value {
	return Value{kind: valueNull}
}

// IsNull will return true if the value is null, and false otherwise.
func (value Value) IsNull() bool {
	return value.kind == valueNull
}

// ---

func (value Value) isCallable() bool {
	switch value := value.value.(type) {
	case *_object:
		return value.isCall()
	}
	return false
}

// Call the value as a function with the given this value and argument list and
// return the result of invocation. It is essentially equivalent to:
//
//		value.apply(thisValue, argumentList)
//
// An undefined value and an error will result if:
//
//		1. There is an error during conversion of the argument list
//		2. The value is not actually a function
//		3. An (uncaught) exception is thrown
//
func (value Value) Call(this Value, argumentList ...interface{}) (Value, error) {
	result := Value{}
	err := catchPanic(func() {
		// FIXME
		result = value.call(nil, this, argumentList...)
	})
	if !value.safe() {
		value = Value{}
	}
	return result, err
}

func (value Value) call(rt *_runtime, this Value, argumentList ...interface{}) Value {
	switch function := value.value.(type) {
	case *_object:
		return function.call(this, function.runtime.toValueArray(argumentList...), false, nativeFrame)
	}
	if rt == nil {
		panic("FIXME TypeError")
	}
	panic(rt.panicTypeError())
}

func (value Value) constructSafe(rt *_runtime, this Value, argumentList ...interface{}) (Value, error) {
	result := Value{}
	err := catchPanic(func() {
		result = value.construct(rt, this, argumentList...)
	})
	return result, err
}

func (value Value) construct(rt *_runtime, this Value, argumentList ...interface{}) Value {
	switch fn := value.value.(type) {
	case *_object:
		return fn.construct(fn.runtime.toValueArray(argumentList...))
	}
	if rt == nil {
		panic("FIXME TypeError")
	}
	panic(rt.panicTypeError())
}

// IsPrimitive will return true if value is a primitive (any kind of primitive).
func (value Value) IsPrimitive() bool {
	return !value.IsObject()
}

// IsBoolean will return true if value is a boolean (primitive).
func (value Value) IsBoolean() bool {
	return value.kind == valueBoolean
}

// IsNumber will return true if value is a number (primitive).
func (value Value) IsNumber() bool {
	return value.kind == valueNumber
}

// IsNaN will return true if value is NaN (or would convert to NaN).
func (value Value) IsNaN() bool {
	switch value := value.value.(type) {
	case float64:
		return math.IsNaN(value)
	case float32:
		return math.IsNaN(float64(value))
	case int, int8, int32, int64:
		return false
	case uint, uint8, uint32, uint64:
		return false
	}

	return math.IsNaN(value.float64())
}

// IsString will return true if value is a string (primitive).
func (value Value) IsString() bool {
	return value.kind == valueString
}

// IsObject will return true if value is an object.
func (value Value) IsObject() bool {
	return value.kind == valueObject
}

// IsFunction will return true if value is a function.
func (value Value) IsFunction() bool {
	if value.kind != valueObject {
		return false
	}
	return value.value.(*_object).class == "Function"
}

// Class will return the class string of the value or the empty string if value is not an object.
//
// The return value will (generally) be one of:
//
//		Object
//		Function
//		Array
//		String
//		Number
//		Boolean
//		Date
//		RegExp
//
func (value Value) Class() string {
	if value.kind != valueObject {
		return ""
	}
	return value.value.(*_object).class
}

func (value Value) isArray() bool {
	if value.kind != valueObject {
		return false
	}
	return isArray(value.value.(*_object))
}

func (value Value) isStringObject() bool {
	if value.kind != valueObject {
		return false
	}
	return value.value.(*_object).class == "String"
}

func (value Value) isBooleanObject() bool {
	if value.kind != valueObject {
		return false
	}
	return value.value.(*_object).class == "Boolean"
}

func (value Value) isNumberObject() bool {
	if value.kind != valueObject {
		return false
	}
	return value.value.(*_object).class == "Number"
}

func (value Value) isDate() bool {
	if value.kind != valueObject {
		return false
	}
	return value.value.(*_object).class == "Date"
}

func (value Value) isRegExp() bool {
	if value.kind != valueObject {
		return false
	}
	return value.value.(*_object).class == "RegExp"
}

func (value Value) isError() bool {
	if value.kind != valueObject {
		return false
	}
	return value.value.(*_object).class == "Error"
}

// ---

func toValue_reflectValuePanic(value interface{}, kind reflect.Kind) {
	// FIXME?
	switch kind {
	case reflect.Struct:
		panic(newError(nil, "TypeError", 0, "invalid value (struct): missing runtime: %v (%T)", value, value))
	case reflect.Map:
		panic(newError(nil, "TypeError", 0, "invalid value (map): missing runtime: %v (%T)", value, value))
	case reflect.Slice:
		panic(newError(nil, "TypeError", 0, "invalid value (slice): missing runtime: %v (%T)", value, value))
	}
}

func toValue(value interface{}) Value {
	switch value := value.(type) {
	case Value:
		return value
	case bool:
		return Value{valueBoolean, value}
	case int:
		return Value{valueNumber, value}
	case int8:
		return Value{valueNumber, value}
	case int16:
		return Value{valueNumber, value}
	case int32:
		return Value{valueNumber, value}
	case int64:
		return Value{valueNumber, value}
	case uint:
		return Value{valueNumber, value}
	case uint8:
		return Value{valueNumber, value}
	case uint16:
		return Value{valueNumber, value}
	case uint32:
		return Value{valueNumber, value}
	case uint64:
		return Value{valueNumber, value}
	case float32:
		return Value{valueNumber, float64(value)}
	case float64:
		return Value{valueNumber, value}
	case []uint16:
		return Value{valueString, value}
	case string:
		return Value{valueString, value}
	// A rune is actually an int32, which is handled above
	case *_object:
		return Value{valueObject, value}
	case *Object:
		return Value{valueObject, value.object}
	case Object:
		return Value{valueObject, value.object}
	case _reference: // reference is an interface (already a pointer)
		return Value{valueReference, value}
	case _result:
		return Value{valueResult, value}
	case nil:
		// TODO Ugh.
		return Value{}
	case reflect.Value:
		for value.Kind() == reflect.Ptr {
			// We were given a pointer, so we'll drill down until we get a non-pointer
			//
			// These semantics might change if we want to start supporting pointers to values transparently
			// (It would be best not to depend on this behavior)
			// FIXME: UNDEFINED
			if value.IsNil() {
				return Value{}
			}
			value = value.Elem()
		}
		switch value.Kind() {
		case reflect.Bool:
			return Value{valueBoolean, bool(value.Bool())}
		case reflect.Int:
			return Value{valueNumber, int(value.Int())}
		case reflect.Int8:
			return Value{valueNumber, int8(value.Int())}
		case reflect.Int16:
			return Value{valueNumber, int16(value.Int())}
		case reflect.Int32:
			return Value{valueNumber, int32(value.Int())}
		case reflect.Int64:
			return Value{valueNumber, int64(value.Int())}
		case reflect.Uint:
			return Value{valueNumber, uint(value.Uint())}
		case reflect.Uint8:
			return Value{valueNumber, uint8(value.Uint())}
		case reflect.Uint16:
			return Value{valueNumber, uint16(value.Uint())}
		case reflect.Uint32:
			return Value{valueNumber, uint32(value.Uint())}
		case reflect.Uint64:
			return Value{valueNumber, uint64(value.Uint())}
		case reflect.Float32:
			return Value{valueNumber, float32(value.Float())}
		case reflect.Float64:
			return Value{valueNumber, float64(value.Float())}
		case reflect.String:
			return Value{valueString, string(value.String())}
		default:
			toValue_reflectValuePanic(value.Interface(), value.Kind())
		}
	default:
		return toValue(reflect.ValueOf(value))
	}
	// FIXME?
	panic(newError(nil, "TypeError", 0, "invalid value: %v (%T)", value, value))
}

// String will return the value as a string.
//
// This method will make return the empty string if there is an error.
func (value Value) String() string {
	result := ""
	catchPanic(func() {
		result = value.string()
	})
	return result
}

// ToBoolean will convert the value to a boolean (bool).
//
//		ToValue(0).ToBoolean() => false
//		ToValue("").ToBoolean() => false
//		ToValue(true).ToBoolean() => true
//		ToValue(1).ToBoolean() => true
//		ToValue("Nothing happens").ToBoolean() => true
//
// If there is an error during the conversion process (like an uncaught exception), then the result will be false and an error.
func (value Value) ToBoolean() (bool, error) {
	result := false
	err := catchPanic(func() {
		result = value.bool()
	})
	return result, err
}

func (value Value) numberValue() Value {
	if value.kind == valueNumber {
		return value
	}
	return Value{valueNumber, value.float64()}
}

// ToFloat will convert the value to a number (float64).
//
//		ToValue(0).ToFloat() => 0.
//		ToValue(1.1).ToFloat() => 1.1
//		ToValue("11").ToFloat() => 11.
//
// If there is an error during the conversion process (like an uncaught exception), then the result will be 0 and an error.
func (value Value) ToFloat() (float64, error) {
	result := float64(0)
	err := catchPanic(func() {
		result = value.float64()
	})
	return result, err
}

// ToInteger will convert the value to a number (int64).
//
//		ToValue(0).ToInteger() => 0
//		ToValue(1.1).ToInteger() => 1
//		ToValue("11").ToInteger() => 11
//
// If there is an error during the conversion process (like an uncaught exception), then the result will be 0 and an error.
func (value Value) ToInteger() (int64, error) {
	result := int64(0)
	err := catchPanic(func() {
		result = value.number().int64
	})
	return result, err
}

// ToString will convert the value to a string (string).
//
//		ToValue(0).ToString() => "0"
//		ToValue(false).ToString() => "false"
//		ToValue(1.1).ToString() => "1.1"
//		ToValue("11").ToString() => "11"
//		ToValue('Nothing happens.').ToString() => "Nothing happens."
//
// If there is an error during the conversion process (like an uncaught exception), then the result will be the empty string ("") and an error.
func (value Value) ToString() (string, error) {
	result := ""
	err := catchPanic(func() {
		result = value.string()
	})
	return result, err
}

func (value Value) _object() *_object {
	switch value := value.value.(type) {
	case *_object:
		return value
	}
	return nil
}

// Object will return the object of the value, or nil if value is not an object.
//
// This method will not do any implicit conversion. For example, calling this method on a string primitive value will not return a String object.
func (value Value) Object() *Object {
	switch object := value.value.(type) {
	case *_object:
		return _newObject(object, value)
	}
	return nil
}

func (value Value) reference() _reference {
	switch value := value.value.(type) {
	case _reference:
		return value
	}
	return nil
}

func (value Value) resolve() Value {
	switch value := value.value.(type) {
	case _reference:
		return value.getValue()
	}
	return value
}

var (
	__NaN__              float64 = math.NaN()
	__PositiveInfinity__ float64 = math.Inf(+1)
	__NegativeInfinity__ float64 = math.Inf(-1)
	__PositiveZero__     float64 = 0
	__NegativeZero__     float64 = math.Float64frombits(0 | (1 << 63))
)

func positiveInfinity() float64 {
	return __PositiveInfinity__
}

func negativeInfinity() float64 {
	return __NegativeInfinity__
}

func positiveZero() float64 {
	return __PositiveZero__
}

func negativeZero() float64 {
	return __NegativeZero__
}

// NaNValue will return a value representing NaN.
//
// It is equivalent to:
//
//		ToValue(math.NaN())
//
func NaNValue() Value {
	return Value{valueNumber, __NaN__}
}

func positiveInfinityValue() Value {
	return Value{valueNumber, __PositiveInfinity__}
}

func negativeInfinityValue() Value {
	return Value{valueNumber, __NegativeInfinity__}
}

func positiveZeroValue() Value {
	return Value{valueNumber, __PositiveZero__}
}

func negativeZeroValue() Value {
	return Value{valueNumber, __NegativeZero__}
}

// TrueValue will return a value representing true.
//
// It is equivalent to:
//
//		ToValue(true)
//
func TrueValue() Value {
	return Value{valueBoolean, true}
}

// FalseValue will return a value representing false.
//
// It is equivalent to:
//
//		ToValue(false)
//
func FalseValue() Value {
	return Value{valueBoolean, false}
}

func sameValue(x Value, y Value) bool {
	if x.kind != y.kind {
		return false
	}
	result := false
	switch x.kind {
	case valueUndefined, valueNull:
		result = true
	case valueNumber:
		x := x.float64()
		y := y.float64()
		if math.IsNaN(x) && math.IsNaN(y) {
			result = true
		} else {
			result = x == y
			if result && x == 0 {
				// Since +0 != -0
				result = math.Signbit(x) == math.Signbit(y)
			}
		}
	case valueString:
		result = x.string() == y.string()
	case valueBoolean:
		result = x.bool() == y.bool()
	case valueObject:
		result = x._object() == y._object()
	default:
		panic(hereBeDragons())
	}

	return result
}

func strictEqualityComparison(x Value, y Value) bool {
	if x.kind != y.kind {
		return false
	}
	result := false
	switch x.kind {
	case valueUndefined, valueNull:
		result = true
	case valueNumber:
		x := x.float64()
		y := y.float64()
		if math.IsNaN(x) && math.IsNaN(y) {
			result = false
		} else {
			result = x == y
		}
	case valueString:
		result = x.string() == y.string()
	case valueBoolean:
		result = x.bool() == y.bool()
	case valueObject:
		result = x._object() == y._object()
	default:
		panic(hereBeDragons())
	}

	return result
}

// Export will attempt to convert the value to a Go representation
// and return it via an interface{} kind.
//
// Export returns an error, but it will always be nil. It is present
// for backwards compatibility.
//
// If a reasonable conversion is not possible, then the original
// value is returned.
//
//      undefined   -> nil (FIXME?: Should be Value{})
//      null        -> nil
//      boolean     -> bool
//      number      -> A number type (int, float32, uint64, ...)
//      string      -> string
//      Array       -> []interface{}
//      Object      -> map[string]interface{}
//
func (self Value) Export() (interface{}, error) {
	return self.export(), nil
}

func (self Value) export() interface{} {

	switch self.kind {
	case valueUndefined:
		return nil
	case valueNull:
		return nil
	case valueNumber, valueBoolean:
		return self.value
	case valueString:
		switch value := self.value.(type) {
		case string:
			return value
		case []uint16:
			return string(utf16.Decode(value))
		}
	case valueObject:
		object := self._object()
		switch value := object.value.(type) {
		case *_goStructObject:
			return value.value.Interface()
		case *_goMapObject:
			return value.value.Interface()
		case *_goArrayObject:
			return value.value.Interface()
		case *_goSliceObject:
			return value.value.Interface()
		}
		if object.class == "Array" {
			result := make([]interface{}, 0)
			lengthValue := object.get("length")
			length := lengthValue.value.(uint32)
			kind := reflect.Invalid
			state := 0
			var t reflect.Type
			for index := uint32(0); index < length; index += 1 {
				name := strconv.FormatInt(int64(index), 10)
				if !object.hasProperty(name) {
					continue
				}
				value := object.get(name).export()

				t = reflect.TypeOf(value)

				var k reflect.Kind
				if t != nil {
					k = t.Kind()
				}

				if state == 0 {
					kind = k
					state = 1
				} else if state == 1 && kind != k {
					state = 2
				}

				result = append(result, value)
			}

			if state != 1 || kind == reflect.Interface || t == nil {
				// No common type
				return result
			}

			// Convert to the common type
			val := reflect.MakeSlice(reflect.SliceOf(t), len(result), len(result))
			for i, v := range result {
				val.Index(i).Set(reflect.ValueOf(v))
			}
			return val.Interface()
		} else {
			result := make(map[string]interface{})
			// TODO Should we export everything? Or just what is enumerable?
			object.enumerate(false, func(name string) bool {
				value := object.get(name)
				if value.IsDefined() {
					result[name] = value.export()
				}
				return true
			})
			return result
		}
	}

	if self.safe() {
		return self
	}

	return Value{}
}

func (self Value) evaluateBreakContinue(labels []string) _resultKind {
	result := self.value.(_result)
	if result.kind == resultBreak || result.kind == resultContinue {
		for _, label := range labels {
			if label == result.target {
				return result.kind
			}
		}
	}
	return resultReturn
}

func (self Value) evaluateBreak(labels []string) _resultKind {
	result := self.value.(_result)
	if result.kind == resultBreak {
		for _, label := range labels {
			if label == result.target {
				return result.kind
			}
		}
	}
	return resultReturn
}

func (self Value) exportNative() interface{} {

	switch self.kind {
	case valueUndefined:
		return self
	case valueNull:
		return nil
	case valueNumber, valueBoolean:
		return self.value
	case valueString:
		switch value := self.value.(type) {
		case string:
			return value
		case []uint16:
			return string(utf16.Decode(value))
		}
	case valueObject:
		object := self._object()
		switch value := object.value.(type) {
		case *_goStructObject:
			return value.value.Interface()
		case *_goMapObject:
			return value.value.Interface()
		case *_goArrayObject:
			return value.value.Interface()
		case *_goSliceObject:
			return value.value.Interface()
		}
	}

	return self
}

// Make a best effort to return a reflect.Value corresponding to reflect.Kind, but
// fallback to just returning the Go value we have handy.
func (value Value) toReflectValue(kind reflect.Kind) (reflect.Value, error) {
	if kind != reflect.Float32 && kind != reflect.Float64 && kind != reflect.Interface {
		switch value := value.value.(type) {
		case float32:
			_, frac := math.Modf(float64(value))
			if frac > 0 {
				return reflect.Value{}, fmt.Errorf("RangeError: %v to reflect.Kind: %v", value, kind)
			}
		case float64:
			_, frac := math.Modf(value)
			if frac > 0 {
				return reflect.Value{}, fmt.Errorf("RangeError: %v to reflect.Kind: %v", value, kind)
			}
		}
	}

	switch kind {
	case reflect.Bool: // Bool
		return reflect.ValueOf(value.bool()), nil
	case reflect.Int: // Int
		// We convert to float64 here because converting to int64 will not tell us
		// if a value is outside the range of int64
		tmp := toIntegerFloat(value)
		if tmp < float_minInt || tmp > float_maxInt {
			return reflect.Value{}, fmt.Errorf("RangeError: %f (%v) to int", tmp, value)
		} else {
			return reflect.ValueOf(int(tmp)), nil
		}
	case reflect.Int8: // Int8
		tmp := value.number().int64
		if tmp < int64_minInt8 || tmp > int64_maxInt8 {
			return reflect.Value{}, fmt.Errorf("RangeError: %d (%v) to int8", tmp, value)
		} else {
			return reflect.ValueOf(int8(tmp)), nil
		}
	case reflect.Int16: // Int16
		tmp := value.number().int64
		if tmp < int64_minInt16 || tmp > int64_maxInt16 {
			return reflect.Value{}, fmt.Errorf("RangeError: %d (%v) to int16", tmp, value)
		} else {
			return reflect.ValueOf(int16(tmp)), nil
		}
	case reflect.Int32: // Int32
		tmp := value.number().int64
		if tmp < int64_minInt32 || tmp > int64_maxInt32 {
			return reflect.Value{}, fmt.Errorf("RangeError: %d (%v) to int32", tmp, value)
		} else {
			return reflect.ValueOf(int32(tmp)), nil
		}
	case reflect.Int64: // Int64
		// We convert to float64 here because converting to int64 will not tell us
		// if a value is outside the range of int64
		tmp := toIntegerFloat(value)
		if tmp < float_minInt64 || tmp > float_maxInt64 {
			return reflect.Value{}, fmt.Errorf("RangeError: %f (%v) to int", tmp, value)
		} else {
			return reflect.ValueOf(int64(tmp)), nil
		}
	case reflect.Uint: // Uint
		// We convert to float64 here because converting to int64 will not tell us
		// if a value is outside the range of uint
		tmp := toIntegerFloat(value)
		if tmp < 0 || tmp > float_maxUint {
			return reflect.Value{}, fmt.Errorf("RangeError: %f (%v) to uint", tmp, value)
		} else {
			return reflect.ValueOf(uint(tmp)), nil
		}
	case reflect.Uint8: // Uint8
		tmp := value.number().int64
		if tmp < 0 || tmp > int64_maxUint8 {
			return reflect.Value{}, fmt.Errorf("RangeError: %d (%v) to uint8", tmp, value)
		} else {
			return reflect.ValueOf(uint8(tmp)), nil
		}
	case reflect.Uint16: // Uint16
		tmp := value.number().int64
		if tmp < 0 || tmp > int64_maxUint16 {
			return reflect.Value{}, fmt.Errorf("RangeError: %d (%v) to uint16", tmp, value)
		} else {
			return reflect.ValueOf(uint16(tmp)), nil
		}
	case reflect.Uint32: // Uint32
		tmp := value.number().int64
		if tmp < 0 || tmp > int64_maxUint32 {
			return reflect.Value{}, fmt.Errorf("RangeError: %d (%v) to uint32", tmp, value)
		} else {
			return reflect.ValueOf(uint32(tmp)), nil
		}
	case reflect.Uint64: // Uint64
		// We convert to float64 here because converting to int64 will not tell us
		// if a value is outside the range of uint64
		tmp := toIntegerFloat(value)
		if tmp < 0 || tmp > float_maxUint64 {
			return reflect.Value{}, fmt.Errorf("RangeError: %f (%v) to uint64", tmp, value)
		} else {
			return reflect.ValueOf(uint64(tmp)), nil
		}
	case reflect.Float32: // Float32
		tmp := value.float64()
		tmp1 := tmp
		if 0 > tmp1 {
			tmp1 = -tmp1
		}
		if tmp1 > 0 && (tmp1 < math.SmallestNonzeroFloat32 || tmp1 > math.MaxFloat32) {
			return reflect.Value{}, fmt.Errorf("RangeError: %f (%v) to float32", tmp, value)
		} else {
			return reflect.ValueOf(float32(tmp)), nil
		}
	case reflect.Float64: // Float64
		value := value.float64()
		return reflect.ValueOf(float64(value)), nil
	case reflect.String: // String
		return reflect.ValueOf(value.string()), nil
	case reflect.Invalid: // Invalid
	case reflect.Complex64: // FIXME? Complex64
	case reflect.Complex128: // FIXME? Complex128
	case reflect.Chan: // FIXME? Chan
	case reflect.Func: // FIXME? Func
	case reflect.Ptr: // FIXME? Ptr
	case reflect.UnsafePointer: // FIXME? UnsafePointer
	default:
		switch value.kind {
		case valueObject:
			object := value._object()
			switch vl := object.value.(type) {
			case *_goStructObject: // Struct
				return reflect.ValueOf(vl.value.Interface()), nil
			case *_goMapObject: // Map
				return reflect.ValueOf(vl.value.Interface()), nil
			case *_goArrayObject: // Array
				return reflect.ValueOf(vl.value.Interface()), nil
			case *_goSliceObject: // Slice
				return reflect.ValueOf(vl.value.Interface()), nil
			}
			return reflect.ValueOf(value.exportNative()), nil
		case valueEmpty, valueResult, valueReference:
			// These are invalid, and should panic
		default:
			return reflect.ValueOf(value.value), nil
		}
	}

	// FIXME Should this end up as a TypeError?
	panic(fmt.Errorf("invalid conversion of %v (%v) to reflect.Kind: %v", value.kind, value, kind))
}

func stringToReflectValue(value string, kind reflect.Kind) (reflect.Value, error) {
	switch kind {
	case reflect.Bool:
		value, err := strconv.ParseBool(value)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(value), nil
	case reflect.Int:
		value, err := strconv.ParseInt(value, 0, 0)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int(value)), nil
	case reflect.Int8:
		value, err := strconv.ParseInt(value, 0, 8)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int8(value)), nil
	case reflect.Int16:
		value, err := strconv.ParseInt(value, 0, 16)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int16(value)), nil
	case reflect.Int32:
		value, err := strconv.ParseInt(value, 0, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int32(value)), nil
	case reflect.Int64:
		value, err := strconv.ParseInt(value, 0, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int64(value)), nil
	case reflect.Uint:
		value, err := strconv.ParseUint(value, 0, 0)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint(value)), nil
	case reflect.Uint8:
		value, err := strconv.ParseUint(value, 0, 8)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint8(value)), nil
	case reflect.Uint16:
		value, err := strconv.ParseUint(value, 0, 16)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint16(value)), nil
	case reflect.Uint32:
		value, err := strconv.ParseUint(value, 0, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint32(value)), nil
	case reflect.Uint64:
		value, err := strconv.ParseUint(value, 0, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint64(value)), nil
	case reflect.Float32:
		value, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(float32(value)), nil
	case reflect.Float64:
		value, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(float64(value)), nil
	case reflect.String:
		return reflect.ValueOf(value), nil
	}

	// FIXME This should end up as a TypeError?
	panic(fmt.Errorf("invalid conversion of %q to reflect.Kind: %v", value, kind))
}
