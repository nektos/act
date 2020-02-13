package otto

import (
	"math"
)

func _newContext(runtime *_runtime) {
	{
		runtime.global.ObjectPrototype = &_object{
			runtime:     runtime,
			class:       "Object",
			objectClass: _classObject,
			prototype:   nil,
			extensible:  true,
			value:       prototypeValueObject,
		}
	}
	{
		runtime.global.FunctionPrototype = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			value:       prototypeValueFunction,
		}
	}
	{
		valueOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "valueOf",
				call: builtinObject_valueOf,
			},
		}
		toString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toString",
				call: builtinObject_toString,
			},
		}
		toLocaleString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toLocaleString",
				call: builtinObject_toLocaleString,
			},
		}
		hasOwnProperty_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "hasOwnProperty",
				call: builtinObject_hasOwnProperty,
			},
		}
		isPrototypeOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "isPrototypeOf",
				call: builtinObject_isPrototypeOf,
			},
		}
		propertyIsEnumerable_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "propertyIsEnumerable",
				call: builtinObject_propertyIsEnumerable,
			},
		}
		runtime.global.ObjectPrototype.property = map[string]_property{
			"valueOf": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: valueOf_function,
				},
			},
			"toString": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: toString_function,
				},
			},
			"toLocaleString": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: toLocaleString_function,
				},
			},
			"hasOwnProperty": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: hasOwnProperty_function,
				},
			},
			"isPrototypeOf": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: isPrototypeOf_function,
				},
			},
			"propertyIsEnumerable": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: propertyIsEnumerable_function,
				},
			},
			"constructor": _property{
				mode:  0101,
				value: Value{},
			},
		}
		runtime.global.ObjectPrototype.propertyOrder = []string{
			"valueOf",
			"toString",
			"toLocaleString",
			"hasOwnProperty",
			"isPrototypeOf",
			"propertyIsEnumerable",
			"constructor",
		}
	}
	{
		toString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toString",
				call: builtinFunction_toString,
			},
		}
		apply_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "apply",
				call: builtinFunction_apply,
			},
		}
		call_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "call",
				call: builtinFunction_call,
			},
		}
		bind_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "bind",
				call: builtinFunction_bind,
			},
		}
		runtime.global.FunctionPrototype.property = map[string]_property{
			"toString": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: toString_function,
				},
			},
			"apply": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: apply_function,
				},
			},
			"call": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: call_function,
				},
			},
			"bind": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: bind_function,
				},
			},
			"constructor": _property{
				mode:  0101,
				value: Value{},
			},
			"length": _property{
				mode: 0,
				value: Value{
					kind:  valueNumber,
					value: 0,
				},
			},
		}
		runtime.global.FunctionPrototype.propertyOrder = []string{
			"toString",
			"apply",
			"call",
			"bind",
			"constructor",
			"length",
		}
	}
	{
		getPrototypeOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getPrototypeOf",
				call: builtinObject_getPrototypeOf,
			},
		}
		getOwnPropertyDescriptor_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getOwnPropertyDescriptor",
				call: builtinObject_getOwnPropertyDescriptor,
			},
		}
		defineProperty_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 3,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "defineProperty",
				call: builtinObject_defineProperty,
			},
		}
		defineProperties_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "defineProperties",
				call: builtinObject_defineProperties,
			},
		}
		create_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "create",
				call: builtinObject_create,
			},
		}
		isExtensible_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "isExtensible",
				call: builtinObject_isExtensible,
			},
		}
		preventExtensions_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "preventExtensions",
				call: builtinObject_preventExtensions,
			},
		}
		isSealed_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "isSealed",
				call: builtinObject_isSealed,
			},
		}
		seal_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "seal",
				call: builtinObject_seal,
			},
		}
		isFrozen_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "isFrozen",
				call: builtinObject_isFrozen,
			},
		}
		freeze_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "freeze",
				call: builtinObject_freeze,
			},
		}
		keys_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "keys",
				call: builtinObject_keys,
			},
		}
		getOwnPropertyNames_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getOwnPropertyNames",
				call: builtinObject_getOwnPropertyNames,
			},
		}
		runtime.global.Object = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "Object",
				call:      builtinObject,
				construct: builtinNewObject,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.ObjectPrototype,
					},
				},
				"getPrototypeOf": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getPrototypeOf_function,
					},
				},
				"getOwnPropertyDescriptor": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getOwnPropertyDescriptor_function,
					},
				},
				"defineProperty": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: defineProperty_function,
					},
				},
				"defineProperties": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: defineProperties_function,
					},
				},
				"create": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: create_function,
					},
				},
				"isExtensible": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: isExtensible_function,
					},
				},
				"preventExtensions": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: preventExtensions_function,
					},
				},
				"isSealed": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: isSealed_function,
					},
				},
				"seal": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: seal_function,
					},
				},
				"isFrozen": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: isFrozen_function,
					},
				},
				"freeze": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: freeze_function,
					},
				},
				"keys": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: keys_function,
					},
				},
				"getOwnPropertyNames": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getOwnPropertyNames_function,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
				"getPrototypeOf",
				"getOwnPropertyDescriptor",
				"defineProperty",
				"defineProperties",
				"create",
				"isExtensible",
				"preventExtensions",
				"isSealed",
				"seal",
				"isFrozen",
				"freeze",
				"keys",
				"getOwnPropertyNames",
			},
		}
		runtime.global.ObjectPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Object,
				},
			}
	}
	{
		Function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "Function",
				call:      builtinFunction,
				construct: builtinNewFunction,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.FunctionPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.Function = Function
		runtime.global.FunctionPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Function,
				},
			}
	}
	{
		toString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toString",
				call: builtinArray_toString,
			},
		}
		toLocaleString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toLocaleString",
				call: builtinArray_toLocaleString,
			},
		}
		concat_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "concat",
				call: builtinArray_concat,
			},
		}
		join_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "join",
				call: builtinArray_join,
			},
		}
		splice_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "splice",
				call: builtinArray_splice,
			},
		}
		shift_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "shift",
				call: builtinArray_shift,
			},
		}
		pop_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "pop",
				call: builtinArray_pop,
			},
		}
		push_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "push",
				call: builtinArray_push,
			},
		}
		slice_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "slice",
				call: builtinArray_slice,
			},
		}
		unshift_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "unshift",
				call: builtinArray_unshift,
			},
		}
		reverse_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "reverse",
				call: builtinArray_reverse,
			},
		}
		sort_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "sort",
				call: builtinArray_sort,
			},
		}
		indexOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "indexOf",
				call: builtinArray_indexOf,
			},
		}
		lastIndexOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "lastIndexOf",
				call: builtinArray_lastIndexOf,
			},
		}
		every_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "every",
				call: builtinArray_every,
			},
		}
		some_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "some",
				call: builtinArray_some,
			},
		}
		forEach_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "forEach",
				call: builtinArray_forEach,
			},
		}
		map_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "map",
				call: builtinArray_map,
			},
		}
		filter_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "filter",
				call: builtinArray_filter,
			},
		}
		reduce_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "reduce",
				call: builtinArray_reduce,
			},
		}
		reduceRight_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "reduceRight",
				call: builtinArray_reduceRight,
			},
		}
		isArray_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "isArray",
				call: builtinArray_isArray,
			},
		}
		runtime.global.ArrayPrototype = &_object{
			runtime:     runtime,
			class:       "Array",
			objectClass: _classArray,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			value:       nil,
			property: map[string]_property{
				"length": _property{
					mode: 0100,
					value: Value{
						kind:  valueNumber,
						value: uint32(0),
					},
				},
				"toString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toString_function,
					},
				},
				"toLocaleString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toLocaleString_function,
					},
				},
				"concat": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: concat_function,
					},
				},
				"join": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: join_function,
					},
				},
				"splice": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: splice_function,
					},
				},
				"shift": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: shift_function,
					},
				},
				"pop": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: pop_function,
					},
				},
				"push": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: push_function,
					},
				},
				"slice": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: slice_function,
					},
				},
				"unshift": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: unshift_function,
					},
				},
				"reverse": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: reverse_function,
					},
				},
				"sort": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: sort_function,
					},
				},
				"indexOf": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: indexOf_function,
					},
				},
				"lastIndexOf": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: lastIndexOf_function,
					},
				},
				"every": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: every_function,
					},
				},
				"some": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: some_function,
					},
				},
				"forEach": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: forEach_function,
					},
				},
				"map": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: map_function,
					},
				},
				"filter": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: filter_function,
					},
				},
				"reduce": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: reduce_function,
					},
				},
				"reduceRight": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: reduceRight_function,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"toString",
				"toLocaleString",
				"concat",
				"join",
				"splice",
				"shift",
				"pop",
				"push",
				"slice",
				"unshift",
				"reverse",
				"sort",
				"indexOf",
				"lastIndexOf",
				"every",
				"some",
				"forEach",
				"map",
				"filter",
				"reduce",
				"reduceRight",
			},
		}
		runtime.global.Array = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "Array",
				call:      builtinArray,
				construct: builtinNewArray,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.ArrayPrototype,
					},
				},
				"isArray": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: isArray_function,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
				"isArray",
			},
		}
		runtime.global.ArrayPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Array,
				},
			}
	}
	{
		toString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toString",
				call: builtinString_toString,
			},
		}
		valueOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "valueOf",
				call: builtinString_valueOf,
			},
		}
		charAt_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "charAt",
				call: builtinString_charAt,
			},
		}
		charCodeAt_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "charCodeAt",
				call: builtinString_charCodeAt,
			},
		}
		concat_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "concat",
				call: builtinString_concat,
			},
		}
		indexOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "indexOf",
				call: builtinString_indexOf,
			},
		}
		lastIndexOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "lastIndexOf",
				call: builtinString_lastIndexOf,
			},
		}
		match_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "match",
				call: builtinString_match,
			},
		}
		replace_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "replace",
				call: builtinString_replace,
			},
		}
		search_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "search",
				call: builtinString_search,
			},
		}
		split_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "split",
				call: builtinString_split,
			},
		}
		slice_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "slice",
				call: builtinString_slice,
			},
		}
		substring_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "substring",
				call: builtinString_substring,
			},
		}
		toLowerCase_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toLowerCase",
				call: builtinString_toLowerCase,
			},
		}
		toUpperCase_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toUpperCase",
				call: builtinString_toUpperCase,
			},
		}
		substr_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "substr",
				call: builtinString_substr,
			},
		}
		trim_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "trim",
				call: builtinString_trim,
			},
		}
		trimLeft_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "trimLeft",
				call: builtinString_trimLeft,
			},
		}
		trimRight_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "trimRight",
				call: builtinString_trimRight,
			},
		}
		localeCompare_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "localeCompare",
				call: builtinString_localeCompare,
			},
		}
		toLocaleLowerCase_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toLocaleLowerCase",
				call: builtinString_toLocaleLowerCase,
			},
		}
		toLocaleUpperCase_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toLocaleUpperCase",
				call: builtinString_toLocaleUpperCase,
			},
		}
		fromCharCode_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "fromCharCode",
				call: builtinString_fromCharCode,
			},
		}
		runtime.global.StringPrototype = &_object{
			runtime:     runtime,
			class:       "String",
			objectClass: _classString,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			value:       prototypeValueString,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: int(0),
					},
				},
				"toString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toString_function,
					},
				},
				"valueOf": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: valueOf_function,
					},
				},
				"charAt": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: charAt_function,
					},
				},
				"charCodeAt": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: charCodeAt_function,
					},
				},
				"concat": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: concat_function,
					},
				},
				"indexOf": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: indexOf_function,
					},
				},
				"lastIndexOf": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: lastIndexOf_function,
					},
				},
				"match": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: match_function,
					},
				},
				"replace": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: replace_function,
					},
				},
				"search": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: search_function,
					},
				},
				"split": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: split_function,
					},
				},
				"slice": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: slice_function,
					},
				},
				"substring": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: substring_function,
					},
				},
				"toLowerCase": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toLowerCase_function,
					},
				},
				"toUpperCase": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toUpperCase_function,
					},
				},
				"substr": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: substr_function,
					},
				},
				"trim": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: trim_function,
					},
				},
				"trimLeft": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: trimLeft_function,
					},
				},
				"trimRight": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: trimRight_function,
					},
				},
				"localeCompare": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: localeCompare_function,
					},
				},
				"toLocaleLowerCase": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toLocaleLowerCase_function,
					},
				},
				"toLocaleUpperCase": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toLocaleUpperCase_function,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"toString",
				"valueOf",
				"charAt",
				"charCodeAt",
				"concat",
				"indexOf",
				"lastIndexOf",
				"match",
				"replace",
				"search",
				"split",
				"slice",
				"substring",
				"toLowerCase",
				"toUpperCase",
				"substr",
				"trim",
				"trimLeft",
				"trimRight",
				"localeCompare",
				"toLocaleLowerCase",
				"toLocaleUpperCase",
			},
		}
		runtime.global.String = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "String",
				call:      builtinString,
				construct: builtinNewString,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.StringPrototype,
					},
				},
				"fromCharCode": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: fromCharCode_function,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
				"fromCharCode",
			},
		}
		runtime.global.StringPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.String,
				},
			}
	}
	{
		toString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toString",
				call: builtinBoolean_toString,
			},
		}
		valueOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "valueOf",
				call: builtinBoolean_valueOf,
			},
		}
		runtime.global.BooleanPrototype = &_object{
			runtime:     runtime,
			class:       "Boolean",
			objectClass: _classObject,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			value:       prototypeValueBoolean,
			property: map[string]_property{
				"toString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toString_function,
					},
				},
				"valueOf": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: valueOf_function,
					},
				},
			},
			propertyOrder: []string{
				"toString",
				"valueOf",
			},
		}
		runtime.global.Boolean = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "Boolean",
				call:      builtinBoolean,
				construct: builtinNewBoolean,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.BooleanPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.BooleanPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Boolean,
				},
			}
	}
	{
		toString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toString",
				call: builtinNumber_toString,
			},
		}
		valueOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "valueOf",
				call: builtinNumber_valueOf,
			},
		}
		toFixed_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toFixed",
				call: builtinNumber_toFixed,
			},
		}
		toExponential_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toExponential",
				call: builtinNumber_toExponential,
			},
		}
		toPrecision_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toPrecision",
				call: builtinNumber_toPrecision,
			},
		}
		toLocaleString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toLocaleString",
				call: builtinNumber_toLocaleString,
			},
		}
		isNaN_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "isNaN",
				call: builtinNumber_isNaN,
			},
		}
		runtime.global.NumberPrototype = &_object{
			runtime:     runtime,
			class:       "Number",
			objectClass: _classObject,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			value:       prototypeValueNumber,
			property: map[string]_property{
				"toString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toString_function,
					},
				},
				"valueOf": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: valueOf_function,
					},
				},
				"toFixed": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toFixed_function,
					},
				},
				"toExponential": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toExponential_function,
					},
				},
				"toPrecision": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toPrecision_function,
					},
				},
				"toLocaleString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toLocaleString_function,
					},
				},
			},
			propertyOrder: []string{
				"toString",
				"valueOf",
				"toFixed",
				"toExponential",
				"toPrecision",
				"toLocaleString",
			},
		}
		runtime.global.Number = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "Number",
				call:      builtinNumber,
				construct: builtinNewNumber,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.NumberPrototype,
					},
				},
				"isNaN": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: isNaN_function,
					},
				},
				"MAX_VALUE": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.MaxFloat64,
					},
				},
				"MIN_VALUE": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.SmallestNonzeroFloat64,
					},
				},
				"NaN": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.NaN(),
					},
				},
				"NEGATIVE_INFINITY": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.Inf(-1),
					},
				},
				"POSITIVE_INFINITY": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.Inf(+1),
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
				"isNaN",
				"MAX_VALUE",
				"MIN_VALUE",
				"NaN",
				"NEGATIVE_INFINITY",
				"POSITIVE_INFINITY",
			},
		}
		runtime.global.NumberPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Number,
				},
			}
	}
	{
		abs_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "abs",
				call: builtinMath_abs,
			},
		}
		acos_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "acos",
				call: builtinMath_acos,
			},
		}
		asin_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "asin",
				call: builtinMath_asin,
			},
		}
		atan_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "atan",
				call: builtinMath_atan,
			},
		}
		atan2_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "atan2",
				call: builtinMath_atan2,
			},
		}
		ceil_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "ceil",
				call: builtinMath_ceil,
			},
		}
		cos_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "cos",
				call: builtinMath_cos,
			},
		}
		exp_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "exp",
				call: builtinMath_exp,
			},
		}
		floor_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "floor",
				call: builtinMath_floor,
			},
		}
		log_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "log",
				call: builtinMath_log,
			},
		}
		max_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "max",
				call: builtinMath_max,
			},
		}
		min_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "min",
				call: builtinMath_min,
			},
		}
		pow_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "pow",
				call: builtinMath_pow,
			},
		}
		random_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "random",
				call: builtinMath_random,
			},
		}
		round_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "round",
				call: builtinMath_round,
			},
		}
		sin_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "sin",
				call: builtinMath_sin,
			},
		}
		sqrt_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "sqrt",
				call: builtinMath_sqrt,
			},
		}
		tan_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "tan",
				call: builtinMath_tan,
			},
		}
		runtime.global.Math = &_object{
			runtime:     runtime,
			class:       "Math",
			objectClass: _classObject,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			property: map[string]_property{
				"abs": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: abs_function,
					},
				},
				"acos": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: acos_function,
					},
				},
				"asin": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: asin_function,
					},
				},
				"atan": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: atan_function,
					},
				},
				"atan2": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: atan2_function,
					},
				},
				"ceil": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: ceil_function,
					},
				},
				"cos": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: cos_function,
					},
				},
				"exp": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: exp_function,
					},
				},
				"floor": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: floor_function,
					},
				},
				"log": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: log_function,
					},
				},
				"max": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: max_function,
					},
				},
				"min": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: min_function,
					},
				},
				"pow": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: pow_function,
					},
				},
				"random": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: random_function,
					},
				},
				"round": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: round_function,
					},
				},
				"sin": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: sin_function,
					},
				},
				"sqrt": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: sqrt_function,
					},
				},
				"tan": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: tan_function,
					},
				},
				"E": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.E,
					},
				},
				"LN10": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.Ln10,
					},
				},
				"LN2": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.Ln2,
					},
				},
				"LOG2E": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.Log2E,
					},
				},
				"LOG10E": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.Log10E,
					},
				},
				"PI": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.Pi,
					},
				},
				"SQRT1_2": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: sqrt1_2,
					},
				},
				"SQRT2": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: math.Sqrt2,
					},
				},
			},
			propertyOrder: []string{
				"abs",
				"acos",
				"asin",
				"atan",
				"atan2",
				"ceil",
				"cos",
				"exp",
				"floor",
				"log",
				"max",
				"min",
				"pow",
				"random",
				"round",
				"sin",
				"sqrt",
				"tan",
				"E",
				"LN10",
				"LN2",
				"LOG2E",
				"LOG10E",
				"PI",
				"SQRT1_2",
				"SQRT2",
			},
		}
	}
	{
		toString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toString",
				call: builtinDate_toString,
			},
		}
		toDateString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toDateString",
				call: builtinDate_toDateString,
			},
		}
		toTimeString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toTimeString",
				call: builtinDate_toTimeString,
			},
		}
		toUTCString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toUTCString",
				call: builtinDate_toUTCString,
			},
		}
		toISOString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toISOString",
				call: builtinDate_toISOString,
			},
		}
		toJSON_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toJSON",
				call: builtinDate_toJSON,
			},
		}
		toGMTString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toGMTString",
				call: builtinDate_toGMTString,
			},
		}
		toLocaleString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toLocaleString",
				call: builtinDate_toLocaleString,
			},
		}
		toLocaleDateString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toLocaleDateString",
				call: builtinDate_toLocaleDateString,
			},
		}
		toLocaleTimeString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toLocaleTimeString",
				call: builtinDate_toLocaleTimeString,
			},
		}
		valueOf_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "valueOf",
				call: builtinDate_valueOf,
			},
		}
		getTime_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getTime",
				call: builtinDate_getTime,
			},
		}
		getYear_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getYear",
				call: builtinDate_getYear,
			},
		}
		getFullYear_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getFullYear",
				call: builtinDate_getFullYear,
			},
		}
		getUTCFullYear_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getUTCFullYear",
				call: builtinDate_getUTCFullYear,
			},
		}
		getMonth_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getMonth",
				call: builtinDate_getMonth,
			},
		}
		getUTCMonth_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getUTCMonth",
				call: builtinDate_getUTCMonth,
			},
		}
		getDate_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getDate",
				call: builtinDate_getDate,
			},
		}
		getUTCDate_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getUTCDate",
				call: builtinDate_getUTCDate,
			},
		}
		getDay_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getDay",
				call: builtinDate_getDay,
			},
		}
		getUTCDay_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getUTCDay",
				call: builtinDate_getUTCDay,
			},
		}
		getHours_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getHours",
				call: builtinDate_getHours,
			},
		}
		getUTCHours_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getUTCHours",
				call: builtinDate_getUTCHours,
			},
		}
		getMinutes_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getMinutes",
				call: builtinDate_getMinutes,
			},
		}
		getUTCMinutes_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getUTCMinutes",
				call: builtinDate_getUTCMinutes,
			},
		}
		getSeconds_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getSeconds",
				call: builtinDate_getSeconds,
			},
		}
		getUTCSeconds_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getUTCSeconds",
				call: builtinDate_getUTCSeconds,
			},
		}
		getMilliseconds_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getMilliseconds",
				call: builtinDate_getMilliseconds,
			},
		}
		getUTCMilliseconds_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getUTCMilliseconds",
				call: builtinDate_getUTCMilliseconds,
			},
		}
		getTimezoneOffset_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "getTimezoneOffset",
				call: builtinDate_getTimezoneOffset,
			},
		}
		setTime_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setTime",
				call: builtinDate_setTime,
			},
		}
		setMilliseconds_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setMilliseconds",
				call: builtinDate_setMilliseconds,
			},
		}
		setUTCMilliseconds_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setUTCMilliseconds",
				call: builtinDate_setUTCMilliseconds,
			},
		}
		setSeconds_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setSeconds",
				call: builtinDate_setSeconds,
			},
		}
		setUTCSeconds_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setUTCSeconds",
				call: builtinDate_setUTCSeconds,
			},
		}
		setMinutes_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 3,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setMinutes",
				call: builtinDate_setMinutes,
			},
		}
		setUTCMinutes_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 3,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setUTCMinutes",
				call: builtinDate_setUTCMinutes,
			},
		}
		setHours_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 4,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setHours",
				call: builtinDate_setHours,
			},
		}
		setUTCHours_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 4,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setUTCHours",
				call: builtinDate_setUTCHours,
			},
		}
		setDate_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setDate",
				call: builtinDate_setDate,
			},
		}
		setUTCDate_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setUTCDate",
				call: builtinDate_setUTCDate,
			},
		}
		setMonth_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setMonth",
				call: builtinDate_setMonth,
			},
		}
		setUTCMonth_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setUTCMonth",
				call: builtinDate_setUTCMonth,
			},
		}
		setYear_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setYear",
				call: builtinDate_setYear,
			},
		}
		setFullYear_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 3,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setFullYear",
				call: builtinDate_setFullYear,
			},
		}
		setUTCFullYear_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 3,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "setUTCFullYear",
				call: builtinDate_setUTCFullYear,
			},
		}
		parse_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "parse",
				call: builtinDate_parse,
			},
		}
		UTC_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 7,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "UTC",
				call: builtinDate_UTC,
			},
		}
		now_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "now",
				call: builtinDate_now,
			},
		}
		runtime.global.DatePrototype = &_object{
			runtime:     runtime,
			class:       "Date",
			objectClass: _classObject,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			value:       prototypeValueDate,
			property: map[string]_property{
				"toString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toString_function,
					},
				},
				"toDateString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toDateString_function,
					},
				},
				"toTimeString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toTimeString_function,
					},
				},
				"toUTCString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toUTCString_function,
					},
				},
				"toISOString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toISOString_function,
					},
				},
				"toJSON": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toJSON_function,
					},
				},
				"toGMTString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toGMTString_function,
					},
				},
				"toLocaleString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toLocaleString_function,
					},
				},
				"toLocaleDateString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toLocaleDateString_function,
					},
				},
				"toLocaleTimeString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toLocaleTimeString_function,
					},
				},
				"valueOf": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: valueOf_function,
					},
				},
				"getTime": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getTime_function,
					},
				},
				"getYear": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getYear_function,
					},
				},
				"getFullYear": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getFullYear_function,
					},
				},
				"getUTCFullYear": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getUTCFullYear_function,
					},
				},
				"getMonth": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getMonth_function,
					},
				},
				"getUTCMonth": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getUTCMonth_function,
					},
				},
				"getDate": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getDate_function,
					},
				},
				"getUTCDate": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getUTCDate_function,
					},
				},
				"getDay": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getDay_function,
					},
				},
				"getUTCDay": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getUTCDay_function,
					},
				},
				"getHours": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getHours_function,
					},
				},
				"getUTCHours": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getUTCHours_function,
					},
				},
				"getMinutes": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getMinutes_function,
					},
				},
				"getUTCMinutes": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getUTCMinutes_function,
					},
				},
				"getSeconds": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getSeconds_function,
					},
				},
				"getUTCSeconds": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getUTCSeconds_function,
					},
				},
				"getMilliseconds": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getMilliseconds_function,
					},
				},
				"getUTCMilliseconds": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getUTCMilliseconds_function,
					},
				},
				"getTimezoneOffset": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: getTimezoneOffset_function,
					},
				},
				"setTime": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setTime_function,
					},
				},
				"setMilliseconds": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setMilliseconds_function,
					},
				},
				"setUTCMilliseconds": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setUTCMilliseconds_function,
					},
				},
				"setSeconds": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setSeconds_function,
					},
				},
				"setUTCSeconds": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setUTCSeconds_function,
					},
				},
				"setMinutes": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setMinutes_function,
					},
				},
				"setUTCMinutes": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setUTCMinutes_function,
					},
				},
				"setHours": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setHours_function,
					},
				},
				"setUTCHours": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setUTCHours_function,
					},
				},
				"setDate": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setDate_function,
					},
				},
				"setUTCDate": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setUTCDate_function,
					},
				},
				"setMonth": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setMonth_function,
					},
				},
				"setUTCMonth": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setUTCMonth_function,
					},
				},
				"setYear": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setYear_function,
					},
				},
				"setFullYear": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setFullYear_function,
					},
				},
				"setUTCFullYear": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: setUTCFullYear_function,
					},
				},
			},
			propertyOrder: []string{
				"toString",
				"toDateString",
				"toTimeString",
				"toUTCString",
				"toISOString",
				"toJSON",
				"toGMTString",
				"toLocaleString",
				"toLocaleDateString",
				"toLocaleTimeString",
				"valueOf",
				"getTime",
				"getYear",
				"getFullYear",
				"getUTCFullYear",
				"getMonth",
				"getUTCMonth",
				"getDate",
				"getUTCDate",
				"getDay",
				"getUTCDay",
				"getHours",
				"getUTCHours",
				"getMinutes",
				"getUTCMinutes",
				"getSeconds",
				"getUTCSeconds",
				"getMilliseconds",
				"getUTCMilliseconds",
				"getTimezoneOffset",
				"setTime",
				"setMilliseconds",
				"setUTCMilliseconds",
				"setSeconds",
				"setUTCSeconds",
				"setMinutes",
				"setUTCMinutes",
				"setHours",
				"setUTCHours",
				"setDate",
				"setUTCDate",
				"setMonth",
				"setUTCMonth",
				"setYear",
				"setFullYear",
				"setUTCFullYear",
			},
		}
		runtime.global.Date = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "Date",
				call:      builtinDate,
				construct: builtinNewDate,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 7,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.DatePrototype,
					},
				},
				"parse": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: parse_function,
					},
				},
				"UTC": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: UTC_function,
					},
				},
				"now": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: now_function,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
				"parse",
				"UTC",
				"now",
			},
		}
		runtime.global.DatePrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Date,
				},
			}
	}
	{
		toString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toString",
				call: builtinRegExp_toString,
			},
		}
		exec_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "exec",
				call: builtinRegExp_exec,
			},
		}
		test_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "test",
				call: builtinRegExp_test,
			},
		}
		compile_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "compile",
				call: builtinRegExp_compile,
			},
		}
		runtime.global.RegExpPrototype = &_object{
			runtime:     runtime,
			class:       "RegExp",
			objectClass: _classObject,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			value:       prototypeValueRegExp,
			property: map[string]_property{
				"toString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toString_function,
					},
				},
				"exec": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: exec_function,
					},
				},
				"test": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: test_function,
					},
				},
				"compile": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: compile_function,
					},
				},
			},
			propertyOrder: []string{
				"toString",
				"exec",
				"test",
				"compile",
			},
		}
		runtime.global.RegExp = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "RegExp",
				call:      builtinRegExp,
				construct: builtinNewRegExp,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.RegExpPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.RegExpPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.RegExp,
				},
			}
	}
	{
		toString_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "toString",
				call: builtinError_toString,
			},
		}
		runtime.global.ErrorPrototype = &_object{
			runtime:     runtime,
			class:       "Error",
			objectClass: _classObject,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			value:       nil,
			property: map[string]_property{
				"toString": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: toString_function,
					},
				},
				"name": _property{
					mode: 0101,
					value: Value{
						kind:  valueString,
						value: "Error",
					},
				},
				"message": _property{
					mode: 0101,
					value: Value{
						kind:  valueString,
						value: "",
					},
				},
			},
			propertyOrder: []string{
				"toString",
				"name",
				"message",
			},
		}
		runtime.global.Error = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "Error",
				call:      builtinError,
				construct: builtinNewError,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.ErrorPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.ErrorPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Error,
				},
			}
	}
	{
		runtime.global.EvalErrorPrototype = &_object{
			runtime:     runtime,
			class:       "EvalError",
			objectClass: _classObject,
			prototype:   runtime.global.ErrorPrototype,
			extensible:  true,
			value:       nil,
			property: map[string]_property{
				"name": _property{
					mode: 0101,
					value: Value{
						kind:  valueString,
						value: "EvalError",
					},
				},
			},
			propertyOrder: []string{
				"name",
			},
		}
		runtime.global.EvalError = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "EvalError",
				call:      builtinEvalError,
				construct: builtinNewEvalError,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.EvalErrorPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.EvalErrorPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.EvalError,
				},
			}
	}
	{
		runtime.global.TypeErrorPrototype = &_object{
			runtime:     runtime,
			class:       "TypeError",
			objectClass: _classObject,
			prototype:   runtime.global.ErrorPrototype,
			extensible:  true,
			value:       nil,
			property: map[string]_property{
				"name": _property{
					mode: 0101,
					value: Value{
						kind:  valueString,
						value: "TypeError",
					},
				},
			},
			propertyOrder: []string{
				"name",
			},
		}
		runtime.global.TypeError = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "TypeError",
				call:      builtinTypeError,
				construct: builtinNewTypeError,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.TypeErrorPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.TypeErrorPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.TypeError,
				},
			}
	}
	{
		runtime.global.RangeErrorPrototype = &_object{
			runtime:     runtime,
			class:       "RangeError",
			objectClass: _classObject,
			prototype:   runtime.global.ErrorPrototype,
			extensible:  true,
			value:       nil,
			property: map[string]_property{
				"name": _property{
					mode: 0101,
					value: Value{
						kind:  valueString,
						value: "RangeError",
					},
				},
			},
			propertyOrder: []string{
				"name",
			},
		}
		runtime.global.RangeError = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "RangeError",
				call:      builtinRangeError,
				construct: builtinNewRangeError,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.RangeErrorPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.RangeErrorPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.RangeError,
				},
			}
	}
	{
		runtime.global.ReferenceErrorPrototype = &_object{
			runtime:     runtime,
			class:       "ReferenceError",
			objectClass: _classObject,
			prototype:   runtime.global.ErrorPrototype,
			extensible:  true,
			value:       nil,
			property: map[string]_property{
				"name": _property{
					mode: 0101,
					value: Value{
						kind:  valueString,
						value: "ReferenceError",
					},
				},
			},
			propertyOrder: []string{
				"name",
			},
		}
		runtime.global.ReferenceError = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "ReferenceError",
				call:      builtinReferenceError,
				construct: builtinNewReferenceError,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.ReferenceErrorPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.ReferenceErrorPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.ReferenceError,
				},
			}
	}
	{
		runtime.global.SyntaxErrorPrototype = &_object{
			runtime:     runtime,
			class:       "SyntaxError",
			objectClass: _classObject,
			prototype:   runtime.global.ErrorPrototype,
			extensible:  true,
			value:       nil,
			property: map[string]_property{
				"name": _property{
					mode: 0101,
					value: Value{
						kind:  valueString,
						value: "SyntaxError",
					},
				},
			},
			propertyOrder: []string{
				"name",
			},
		}
		runtime.global.SyntaxError = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "SyntaxError",
				call:      builtinSyntaxError,
				construct: builtinNewSyntaxError,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.SyntaxErrorPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.SyntaxErrorPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.SyntaxError,
				},
			}
	}
	{
		runtime.global.URIErrorPrototype = &_object{
			runtime:     runtime,
			class:       "URIError",
			objectClass: _classObject,
			prototype:   runtime.global.ErrorPrototype,
			extensible:  true,
			value:       nil,
			property: map[string]_property{
				"name": _property{
					mode: 0101,
					value: Value{
						kind:  valueString,
						value: "URIError",
					},
				},
			},
			propertyOrder: []string{
				"name",
			},
		}
		runtime.global.URIError = &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			value: _nativeFunctionObject{
				name:      "URIError",
				call:      builtinURIError,
				construct: builtinNewURIError,
			},
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
				"prototype": _property{
					mode: 0,
					value: Value{
						kind:  valueObject,
						value: runtime.global.URIErrorPrototype,
					},
				},
			},
			propertyOrder: []string{
				"length",
				"prototype",
			},
		}
		runtime.global.URIErrorPrototype.property["constructor"] =
			_property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.URIError,
				},
			}
	}
	{
		parse_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "parse",
				call: builtinJSON_parse,
			},
		}
		stringify_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 3,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "stringify",
				call: builtinJSON_stringify,
			},
		}
		runtime.global.JSON = &_object{
			runtime:     runtime,
			class:       "JSON",
			objectClass: _classObject,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			property: map[string]_property{
				"parse": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: parse_function,
					},
				},
				"stringify": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: stringify_function,
					},
				},
			},
			propertyOrder: []string{
				"parse",
				"stringify",
			},
		}
	}
	{
		eval_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "eval",
				call: builtinGlobal_eval,
			},
		}
		parseInt_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 2,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "parseInt",
				call: builtinGlobal_parseInt,
			},
		}
		parseFloat_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "parseFloat",
				call: builtinGlobal_parseFloat,
			},
		}
		isNaN_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "isNaN",
				call: builtinGlobal_isNaN,
			},
		}
		isFinite_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "isFinite",
				call: builtinGlobal_isFinite,
			},
		}
		decodeURI_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "decodeURI",
				call: builtinGlobal_decodeURI,
			},
		}
		decodeURIComponent_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "decodeURIComponent",
				call: builtinGlobal_decodeURIComponent,
			},
		}
		encodeURI_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "encodeURI",
				call: builtinGlobal_encodeURI,
			},
		}
		encodeURIComponent_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "encodeURIComponent",
				call: builtinGlobal_encodeURIComponent,
			},
		}
		escape_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "escape",
				call: builtinGlobal_escape,
			},
		}
		unescape_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 1,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "unescape",
				call: builtinGlobal_unescape,
			},
		}
		runtime.globalObject.property = map[string]_property{
			"eval": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: eval_function,
				},
			},
			"parseInt": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: parseInt_function,
				},
			},
			"parseFloat": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: parseFloat_function,
				},
			},
			"isNaN": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: isNaN_function,
				},
			},
			"isFinite": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: isFinite_function,
				},
			},
			"decodeURI": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: decodeURI_function,
				},
			},
			"decodeURIComponent": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: decodeURIComponent_function,
				},
			},
			"encodeURI": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: encodeURI_function,
				},
			},
			"encodeURIComponent": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: encodeURIComponent_function,
				},
			},
			"escape": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: escape_function,
				},
			},
			"unescape": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: unescape_function,
				},
			},
			"Object": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Object,
				},
			},
			"Function": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Function,
				},
			},
			"Array": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Array,
				},
			},
			"String": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.String,
				},
			},
			"Boolean": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Boolean,
				},
			},
			"Number": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Number,
				},
			},
			"Math": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Math,
				},
			},
			"Date": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Date,
				},
			},
			"RegExp": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.RegExp,
				},
			},
			"Error": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.Error,
				},
			},
			"EvalError": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.EvalError,
				},
			},
			"TypeError": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.TypeError,
				},
			},
			"RangeError": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.RangeError,
				},
			},
			"ReferenceError": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.ReferenceError,
				},
			},
			"SyntaxError": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.SyntaxError,
				},
			},
			"URIError": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.URIError,
				},
			},
			"JSON": _property{
				mode: 0101,
				value: Value{
					kind:  valueObject,
					value: runtime.global.JSON,
				},
			},
			"undefined": _property{
				mode: 0,
				value: Value{
					kind: valueUndefined,
				},
			},
			"NaN": _property{
				mode: 0,
				value: Value{
					kind:  valueNumber,
					value: math.NaN(),
				},
			},
			"Infinity": _property{
				mode: 0,
				value: Value{
					kind:  valueNumber,
					value: math.Inf(+1),
				},
			},
		}
		runtime.globalObject.propertyOrder = []string{
			"eval",
			"parseInt",
			"parseFloat",
			"isNaN",
			"isFinite",
			"decodeURI",
			"decodeURIComponent",
			"encodeURI",
			"encodeURIComponent",
			"escape",
			"unescape",
			"Object",
			"Function",
			"Array",
			"String",
			"Boolean",
			"Number",
			"Math",
			"Date",
			"RegExp",
			"Error",
			"EvalError",
			"TypeError",
			"RangeError",
			"ReferenceError",
			"SyntaxError",
			"URIError",
			"JSON",
			"undefined",
			"NaN",
			"Infinity",
		}
	}
}

func newConsoleObject(runtime *_runtime) *_object {
	{
		log_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "log",
				call: builtinConsole_log,
			},
		}
		debug_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "debug",
				call: builtinConsole_log,
			},
		}
		info_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "info",
				call: builtinConsole_log,
			},
		}
		error_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "error",
				call: builtinConsole_error,
			},
		}
		warn_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "warn",
				call: builtinConsole_error,
			},
		}
		dir_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "dir",
				call: builtinConsole_dir,
			},
		}
		time_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "time",
				call: builtinConsole_time,
			},
		}
		timeEnd_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "timeEnd",
				call: builtinConsole_timeEnd,
			},
		}
		trace_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "trace",
				call: builtinConsole_trace,
			},
		}
		assert_function := &_object{
			runtime:     runtime,
			class:       "Function",
			objectClass: _classObject,
			prototype:   runtime.global.FunctionPrototype,
			extensible:  true,
			property: map[string]_property{
				"length": _property{
					mode: 0,
					value: Value{
						kind:  valueNumber,
						value: 0,
					},
				},
			},
			propertyOrder: []string{
				"length",
			},
			value: _nativeFunctionObject{
				name: "assert",
				call: builtinConsole_assert,
			},
		}
		return &_object{
			runtime:     runtime,
			class:       "Object",
			objectClass: _classObject,
			prototype:   runtime.global.ObjectPrototype,
			extensible:  true,
			property: map[string]_property{
				"log": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: log_function,
					},
				},
				"debug": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: debug_function,
					},
				},
				"info": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: info_function,
					},
				},
				"error": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: error_function,
					},
				},
				"warn": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: warn_function,
					},
				},
				"dir": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: dir_function,
					},
				},
				"time": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: time_function,
					},
				},
				"timeEnd": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: timeEnd_function,
					},
				},
				"trace": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: trace_function,
					},
				},
				"assert": _property{
					mode: 0101,
					value: Value{
						kind:  valueObject,
						value: assert_function,
					},
				},
			},
			propertyOrder: []string{
				"log",
				"debug",
				"info",
				"error",
				"warn",
				"dir",
				"time",
				"timeEnd",
				"trace",
				"assert",
			},
		}
	}
}

func toValue_int(value int) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_int8(value int8) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_int16(value int16) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_int32(value int32) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_int64(value int64) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_uint(value uint) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_uint8(value uint8) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_uint16(value uint16) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_uint32(value uint32) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_uint64(value uint64) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_float32(value float32) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_float64(value float64) Value {
	return Value{
		kind:  valueNumber,
		value: value,
	}
}

func toValue_string(value string) Value {
	return Value{
		kind:  valueString,
		value: value,
	}
}

func toValue_string16(value []uint16) Value {
	return Value{
		kind:  valueString,
		value: value,
	}
}

func toValue_bool(value bool) Value {
	return Value{
		kind:  valueBoolean,
		value: value,
	}
}

func toValue_object(value *_object) Value {
	return Value{
		kind:  valueObject,
		value: value,
	}
}
