package otto

import (
	"math"
	"strconv"
)

// Number

func numberValueFromNumberArgumentList(argumentList []Value) Value {
	if len(argumentList) > 0 {
		return argumentList[0].numberValue()
	}
	return toValue_int(0)
}

func builtinNumber(call FunctionCall) Value {
	return numberValueFromNumberArgumentList(call.ArgumentList)
}

func builtinNewNumber(self *_object, argumentList []Value) Value {
	return toValue_object(self.runtime.newNumber(numberValueFromNumberArgumentList(argumentList)))
}

func builtinNumber_toString(call FunctionCall) Value {
	// Will throw a TypeError if ThisObject is not a Number
	value := call.thisClassObject("Number").primitiveValue()
	radix := 10
	radixArgument := call.Argument(0)
	if radixArgument.IsDefined() {
		integer := toIntegerFloat(radixArgument)
		if integer < 2 || integer > 36 {
			panic(call.runtime.panicRangeError("toString() radix must be between 2 and 36"))
		}
		radix = int(integer)
	}
	if radix == 10 {
		return toValue_string(value.string())
	}
	return toValue_string(numberToStringRadix(value, radix))
}

func builtinNumber_valueOf(call FunctionCall) Value {
	return call.thisClassObject("Number").primitiveValue()
}

func builtinNumber_toFixed(call FunctionCall) Value {
	precision := toIntegerFloat(call.Argument(0))
	if 20 < precision || 0 > precision {
		panic(call.runtime.panicRangeError("toFixed() precision must be between 0 and 20"))
	}
	if call.This.IsNaN() {
		return toValue_string("NaN")
	}
	value := call.This.float64()
	if math.Abs(value) >= 1e21 {
		return toValue_string(floatToString(value, 64))
	}
	return toValue_string(strconv.FormatFloat(call.This.float64(), 'f', int(precision), 64))
}

func builtinNumber_toExponential(call FunctionCall) Value {
	if call.This.IsNaN() {
		return toValue_string("NaN")
	}
	precision := float64(-1)
	if value := call.Argument(0); value.IsDefined() {
		precision = toIntegerFloat(value)
		if 0 > precision {
			panic(call.runtime.panicRangeError("toString() radix must be between 2 and 36"))
		}
	}
	return toValue_string(strconv.FormatFloat(call.This.float64(), 'e', int(precision), 64))
}

func builtinNumber_toPrecision(call FunctionCall) Value {
	if call.This.IsNaN() {
		return toValue_string("NaN")
	}
	value := call.Argument(0)
	if value.IsUndefined() {
		return toValue_string(call.This.string())
	}
	precision := toIntegerFloat(value)
	if 1 > precision {
		panic(call.runtime.panicRangeError("toPrecision() precision must be greater than 1"))
	}
	return toValue_string(strconv.FormatFloat(call.This.float64(), 'g', int(precision), 64))
}

func builtinNumber_isNaN(call FunctionCall) Value {
	if len(call.ArgumentList) < 1 {
		return toValue_bool(false)
	}
	return toValue_bool(call.Argument(0).IsNaN())
}

func builtinNumber_toLocaleString(call FunctionCall) Value {
	return builtinNumber_toString(call)
}
