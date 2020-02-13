package otto

// _constructFunction
type _constructFunction func(*_object, []Value) Value

// 13.2.2 [[Construct]]
func defaultConstruct(fn *_object, argumentList []Value) Value {
	object := fn.runtime.newObject()
	object.class = "Object"

	prototype := fn.get("prototype")
	if prototype.kind != valueObject {
		prototype = toValue_object(fn.runtime.global.ObjectPrototype)
	}
	object.prototype = prototype._object()

	this := toValue_object(object)
	value := fn.call(this, argumentList, false, nativeFrame)
	if value.kind == valueObject {
		return value
	}
	return this
}

// _nativeFunction
type _nativeFunction func(FunctionCall) Value

// ===================== //
// _nativeFunctionObject //
// ===================== //

type _nativeFunctionObject struct {
	name      string
	file      string
	line      int
	call      _nativeFunction    // [[Call]]
	construct _constructFunction // [[Construct]]
}

func (runtime *_runtime) _newNativeFunctionObject(name, file string, line int, native _nativeFunction, length int) *_object {
	self := runtime.newClassObject("Function")
	self.value = _nativeFunctionObject{
		name:      name,
		file:      file,
		line:      line,
		call:      native,
		construct: defaultConstruct,
	}
	self.defineProperty("name", toValue_string(name), 0000, false)
	self.defineProperty("length", toValue_int(length), 0000, false)
	return self
}

func (runtime *_runtime) newNativeFunctionObject(name, file string, line int, native _nativeFunction, length int) *_object {
	self := runtime._newNativeFunctionObject(name, file, line, native, length)
	self.defineOwnProperty("caller", _property{
		value: _propertyGetSet{
			runtime._newNativeFunctionObject("get", "internal", 0, func(fc FunctionCall) Value {
				for sc := runtime.scope; sc != nil; sc = sc.outer {
					if sc.frame.fn == self {
						if sc.outer == nil || sc.outer.frame.fn == nil {
							return nullValue
						}

						return runtime.toValue(sc.outer.frame.fn)
					}
				}

				return nullValue
			}, 0),
			&_nilGetSetObject,
		},
		mode: 0000,
	}, false)
	return self
}

// =================== //
// _bindFunctionObject //
// =================== //

type _bindFunctionObject struct {
	target       *_object
	this         Value
	argumentList []Value
}

func (runtime *_runtime) newBoundFunctionObject(target *_object, this Value, argumentList []Value) *_object {
	self := runtime.newClassObject("Function")
	self.value = _bindFunctionObject{
		target:       target,
		this:         this,
		argumentList: argumentList,
	}
	length := int(toInt32(target.get("length")))
	length -= len(argumentList)
	if length < 0 {
		length = 0
	}
	self.defineProperty("name", toValue_string("bound "+target.get("name").String()), 0000, false)
	self.defineProperty("length", toValue_int(length), 0000, false)
	self.defineProperty("caller", Value{}, 0000, false)    // TODO Should throw a TypeError
	self.defineProperty("arguments", Value{}, 0000, false) // TODO Should throw a TypeError
	return self
}

// [[Construct]]
func (fn _bindFunctionObject) construct(argumentList []Value) Value {
	object := fn.target
	switch value := object.value.(type) {
	case _nativeFunctionObject:
		return value.construct(object, fn.argumentList)
	case _nodeFunctionObject:
		argumentList = append(fn.argumentList, argumentList...)
		return object.construct(argumentList)
	}
	panic(fn.target.runtime.panicTypeError())
}

// =================== //
// _nodeFunctionObject //
// =================== //

type _nodeFunctionObject struct {
	node  *_nodeFunctionLiteral
	stash _stash
}

func (runtime *_runtime) newNodeFunctionObject(node *_nodeFunctionLiteral, stash _stash) *_object {
	self := runtime.newClassObject("Function")
	self.value = _nodeFunctionObject{
		node:  node,
		stash: stash,
	}
	self.defineProperty("name", toValue_string(node.name), 0000, false)
	self.defineProperty("length", toValue_int(len(node.parameterList)), 0000, false)
	self.defineOwnProperty("caller", _property{
		value: _propertyGetSet{
			runtime.newNativeFunction("get", "internal", 0, func(fc FunctionCall) Value {
				for sc := runtime.scope; sc != nil; sc = sc.outer {
					if sc.frame.fn == self {
						if sc.outer == nil || sc.outer.frame.fn == nil {
							return nullValue
						}

						return runtime.toValue(sc.outer.frame.fn)
					}
				}

				return nullValue
			}),
			&_nilGetSetObject,
		},
		mode: 0000,
	}, false)
	return self
}

// ======= //
// _object //
// ======= //

func (self *_object) isCall() bool {
	switch fn := self.value.(type) {
	case _nativeFunctionObject:
		return fn.call != nil
	case _bindFunctionObject:
		return true
	case _nodeFunctionObject:
		return true
	}
	return false
}

func (self *_object) call(this Value, argumentList []Value, eval bool, frame _frame) Value {
	switch fn := self.value.(type) {

	case _nativeFunctionObject:
		// Since eval is a native function, we only have to check for it here
		if eval {
			eval = self == self.runtime.eval // If eval is true, then it IS a direct eval
		}

		// Enter a scope, name from the native object...
		rt := self.runtime
		if rt.scope != nil && !eval {
			rt.enterFunctionScope(rt.scope.lexical, this)
			rt.scope.frame = _frame{
				native:     true,
				nativeFile: fn.file,
				nativeLine: fn.line,
				callee:     fn.name,
				file:       nil,
				fn:         self,
			}
			defer func() {
				rt.leaveScope()
			}()
		}

		return fn.call(FunctionCall{
			runtime: self.runtime,
			eval:    eval,

			This:         this,
			ArgumentList: argumentList,
			Otto:         self.runtime.otto,
		})

	case _bindFunctionObject:
		// TODO Passthrough site, do not enter a scope
		argumentList = append(fn.argumentList, argumentList...)
		return fn.target.call(fn.this, argumentList, false, frame)

	case _nodeFunctionObject:
		rt := self.runtime
		stash := rt.enterFunctionScope(fn.stash, this)
		rt.scope.frame = _frame{
			callee: fn.node.name,
			file:   fn.node.file,
			fn:     self,
		}
		defer func() {
			rt.leaveScope()
		}()
		callValue := rt.cmpl_call_nodeFunction(self, stash, fn.node, this, argumentList)
		if value, valid := callValue.value.(_result); valid {
			return value.value
		}
		return callValue
	}

	panic(self.runtime.panicTypeError("%v is not a function", toValue_object(self)))
}

func (self *_object) construct(argumentList []Value) Value {
	switch fn := self.value.(type) {

	case _nativeFunctionObject:
		if fn.call == nil {
			panic(self.runtime.panicTypeError("%v is not a function", toValue_object(self)))
		}
		if fn.construct == nil {
			panic(self.runtime.panicTypeError("%v is not a constructor", toValue_object(self)))
		}
		return fn.construct(self, argumentList)

	case _bindFunctionObject:
		return fn.construct(argumentList)

	case _nodeFunctionObject:
		return defaultConstruct(self, argumentList)
	}

	panic(self.runtime.panicTypeError("%v is not a function", toValue_object(self)))
}

// 15.3.5.3
func (self *_object) hasInstance(of Value) bool {
	if !self.isCall() {
		// We should not have a hasInstance method
		panic(self.runtime.panicTypeError())
	}
	if !of.IsObject() {
		return false
	}
	prototype := self.get("prototype")
	if !prototype.IsObject() {
		panic(self.runtime.panicTypeError())
	}
	prototypeObject := prototype._object()

	value := of._object().prototype
	for value != nil {
		if value == prototypeObject {
			return true
		}
		value = value.prototype
	}
	return false
}

// ============ //
// FunctionCall //
// ============ //

// FunctionCall is an encapsulation of a JavaScript function call.
type FunctionCall struct {
	runtime     *_runtime
	_thisObject *_object
	eval        bool // This call is a direct call to eval

	This         Value
	ArgumentList []Value
	Otto         *Otto
}

// Argument will return the value of the argument at the given index.
//
// If no such argument exists, undefined is returned.
func (self FunctionCall) Argument(index int) Value {
	return valueOfArrayIndex(self.ArgumentList, index)
}

func (self FunctionCall) getArgument(index int) (Value, bool) {
	return getValueOfArrayIndex(self.ArgumentList, index)
}

func (self FunctionCall) slice(index int) []Value {
	if index < len(self.ArgumentList) {
		return self.ArgumentList[index:]
	}
	return []Value{}
}

func (self *FunctionCall) thisObject() *_object {
	if self._thisObject == nil {
		this := self.This.resolve() // FIXME Is this right?
		self._thisObject = self.runtime.toObject(this)
	}
	return self._thisObject
}

func (self *FunctionCall) thisClassObject(class string) *_object {
	thisObject := self.thisObject()
	if thisObject.class != class {
		panic(self.runtime.panicTypeError())
	}
	return self._thisObject
}

func (self FunctionCall) toObject(value Value) *_object {
	return self.runtime.toObject(value)
}

// CallerLocation will return file location information (file:line:pos) where this function is being called.
func (self FunctionCall) CallerLocation() string {
	// see error.go for location()
	return self.runtime.scope.outer.frame.location()
}
