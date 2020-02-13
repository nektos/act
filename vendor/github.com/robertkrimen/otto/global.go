package otto

import (
	"strconv"
	"time"
)

var (
	prototypeValueObject   = interface{}(nil)
	prototypeValueFunction = _nativeFunctionObject{
		call: func(_ FunctionCall) Value {
			return Value{}
		},
	}
	prototypeValueString = _stringASCII("")
	// TODO Make this just false?
	prototypeValueBoolean = Value{
		kind:  valueBoolean,
		value: false,
	}
	prototypeValueNumber = Value{
		kind:  valueNumber,
		value: 0,
	}
	prototypeValueDate = _dateObject{
		epoch: 0,
		isNaN: false,
		time:  time.Unix(0, 0).UTC(),
		value: Value{
			kind:  valueNumber,
			value: 0,
		},
	}
	prototypeValueRegExp = _regExpObject{
		regularExpression: nil,
		global:            false,
		ignoreCase:        false,
		multiline:         false,
		source:            "",
		flags:             "",
	}
)

func newContext() *_runtime {

	self := &_runtime{}

	self.globalStash = self.newObjectStash(nil, nil)
	self.globalObject = self.globalStash.object

	_newContext(self)

	self.eval = self.globalObject.property["eval"].value.(Value).value.(*_object)
	self.globalObject.prototype = self.global.ObjectPrototype

	return self
}

func (runtime *_runtime) newBaseObject() *_object {
	self := newObject(runtime, "")
	return self
}

func (runtime *_runtime) newClassObject(class string) *_object {
	return newObject(runtime, class)
}

func (runtime *_runtime) newPrimitiveObject(class string, value Value) *_object {
	self := runtime.newClassObject(class)
	self.value = value
	return self
}

func (self *_object) primitiveValue() Value {
	switch value := self.value.(type) {
	case Value:
		return value
	case _stringObject:
		return toValue_string(value.String())
	}
	return Value{}
}

func (self *_object) hasPrimitive() bool {
	switch self.value.(type) {
	case Value, _stringObject:
		return true
	}
	return false
}

func (runtime *_runtime) newObject() *_object {
	self := runtime.newClassObject("Object")
	self.prototype = runtime.global.ObjectPrototype
	return self
}

func (runtime *_runtime) newArray(length uint32) *_object {
	self := runtime.newArrayObject(length)
	self.prototype = runtime.global.ArrayPrototype
	return self
}

func (runtime *_runtime) newArrayOf(valueArray []Value) *_object {
	self := runtime.newArray(uint32(len(valueArray)))
	for index, value := range valueArray {
		if value.isEmpty() {
			continue
		}
		self.defineProperty(strconv.FormatInt(int64(index), 10), value, 0111, false)
	}
	return self
}

func (runtime *_runtime) newString(value Value) *_object {
	self := runtime.newStringObject(value)
	self.prototype = runtime.global.StringPrototype
	return self
}

func (runtime *_runtime) newBoolean(value Value) *_object {
	self := runtime.newBooleanObject(value)
	self.prototype = runtime.global.BooleanPrototype
	return self
}

func (runtime *_runtime) newNumber(value Value) *_object {
	self := runtime.newNumberObject(value)
	self.prototype = runtime.global.NumberPrototype
	return self
}

func (runtime *_runtime) newRegExp(patternValue Value, flagsValue Value) *_object {

	pattern := ""
	flags := ""
	if object := patternValue._object(); object != nil && object.class == "RegExp" {
		if flagsValue.IsDefined() {
			panic(runtime.panicTypeError("Cannot supply flags when constructing one RegExp from another"))
		}
		regExp := object.regExpValue()
		pattern = regExp.source
		flags = regExp.flags
	} else {
		if patternValue.IsDefined() {
			pattern = patternValue.string()
		}
		if flagsValue.IsDefined() {
			flags = flagsValue.string()
		}
	}

	return runtime._newRegExp(pattern, flags)
}

func (runtime *_runtime) _newRegExp(pattern string, flags string) *_object {
	self := runtime.newRegExpObject(pattern, flags)
	self.prototype = runtime.global.RegExpPrototype
	return self
}

// TODO Should (probably) be one argument, right? This is redundant
func (runtime *_runtime) newDate(epoch float64) *_object {
	self := runtime.newDateObject(epoch)
	self.prototype = runtime.global.DatePrototype
	return self
}

func (runtime *_runtime) newError(name string, message Value, stackFramesToPop int) *_object {
	var self *_object
	switch name {
	case "EvalError":
		return runtime.newEvalError(message)
	case "TypeError":
		return runtime.newTypeError(message)
	case "RangeError":
		return runtime.newRangeError(message)
	case "ReferenceError":
		return runtime.newReferenceError(message)
	case "SyntaxError":
		return runtime.newSyntaxError(message)
	case "URIError":
		return runtime.newURIError(message)
	}

	self = runtime.newErrorObject(name, message, stackFramesToPop)
	self.prototype = runtime.global.ErrorPrototype
	if name != "" {
		self.defineProperty("name", toValue_string(name), 0111, false)
	}
	return self
}

func (runtime *_runtime) newNativeFunction(name, file string, line int, _nativeFunction _nativeFunction) *_object {
	self := runtime.newNativeFunctionObject(name, file, line, _nativeFunction, 0)
	self.prototype = runtime.global.FunctionPrototype
	prototype := runtime.newObject()
	self.defineProperty("prototype", toValue_object(prototype), 0100, false)
	prototype.defineProperty("constructor", toValue_object(self), 0100, false)
	return self
}

func (runtime *_runtime) newNodeFunction(node *_nodeFunctionLiteral, scopeEnvironment _stash) *_object {
	// TODO Implement 13.2 fully
	self := runtime.newNodeFunctionObject(node, scopeEnvironment)
	self.prototype = runtime.global.FunctionPrototype
	prototype := runtime.newObject()
	self.defineProperty("prototype", toValue_object(prototype), 0100, false)
	prototype.defineProperty("constructor", toValue_object(self), 0101, false)
	return self
}

// FIXME Only in one place...
func (runtime *_runtime) newBoundFunction(target *_object, this Value, argumentList []Value) *_object {
	self := runtime.newBoundFunctionObject(target, this, argumentList)
	self.prototype = runtime.global.FunctionPrototype
	prototype := runtime.newObject()
	self.defineProperty("prototype", toValue_object(prototype), 0100, false)
	prototype.defineProperty("constructor", toValue_object(self), 0100, false)
	return self
}
