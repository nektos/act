package otto

import (
	"fmt"
)

type _clone struct {
	runtime      *_runtime
	_object      map[*_object]*_object
	_objectStash map[*_objectStash]*_objectStash
	_dclStash    map[*_dclStash]*_dclStash
	_fnStash     map[*_fnStash]*_fnStash
}

func (in *_runtime) clone() *_runtime {

	in.lck.Lock()
	defer in.lck.Unlock()

	out := &_runtime{
		debugger:   in.debugger,
		random:     in.random,
		stackLimit: in.stackLimit,
		traceLimit: in.traceLimit,
	}

	clone := _clone{
		runtime:      out,
		_object:      make(map[*_object]*_object),
		_objectStash: make(map[*_objectStash]*_objectStash),
		_dclStash:    make(map[*_dclStash]*_dclStash),
		_fnStash:     make(map[*_fnStash]*_fnStash),
	}

	globalObject := clone.object(in.globalObject)
	out.globalStash = out.newObjectStash(globalObject, nil)
	out.globalObject = globalObject
	out.global = _global{
		clone.object(in.global.Object),
		clone.object(in.global.Function),
		clone.object(in.global.Array),
		clone.object(in.global.String),
		clone.object(in.global.Boolean),
		clone.object(in.global.Number),
		clone.object(in.global.Math),
		clone.object(in.global.Date),
		clone.object(in.global.RegExp),
		clone.object(in.global.Error),
		clone.object(in.global.EvalError),
		clone.object(in.global.TypeError),
		clone.object(in.global.RangeError),
		clone.object(in.global.ReferenceError),
		clone.object(in.global.SyntaxError),
		clone.object(in.global.URIError),
		clone.object(in.global.JSON),

		clone.object(in.global.ObjectPrototype),
		clone.object(in.global.FunctionPrototype),
		clone.object(in.global.ArrayPrototype),
		clone.object(in.global.StringPrototype),
		clone.object(in.global.BooleanPrototype),
		clone.object(in.global.NumberPrototype),
		clone.object(in.global.DatePrototype),
		clone.object(in.global.RegExpPrototype),
		clone.object(in.global.ErrorPrototype),
		clone.object(in.global.EvalErrorPrototype),
		clone.object(in.global.TypeErrorPrototype),
		clone.object(in.global.RangeErrorPrototype),
		clone.object(in.global.ReferenceErrorPrototype),
		clone.object(in.global.SyntaxErrorPrototype),
		clone.object(in.global.URIErrorPrototype),
	}

	out.eval = out.globalObject.property["eval"].value.(Value).value.(*_object)
	out.globalObject.prototype = out.global.ObjectPrototype

	// Not sure if this is necessary, but give some help to the GC
	clone.runtime = nil
	clone._object = nil
	clone._objectStash = nil
	clone._dclStash = nil
	clone._fnStash = nil

	return out
}

func (clone *_clone) object(in *_object) *_object {
	if out, exists := clone._object[in]; exists {
		return out
	}
	out := &_object{}
	clone._object[in] = out
	return in.objectClass.clone(in, out, clone)
}

func (clone *_clone) dclStash(in *_dclStash) (*_dclStash, bool) {
	if out, exists := clone._dclStash[in]; exists {
		return out, true
	}
	out := &_dclStash{}
	clone._dclStash[in] = out
	return out, false
}

func (clone *_clone) objectStash(in *_objectStash) (*_objectStash, bool) {
	if out, exists := clone._objectStash[in]; exists {
		return out, true
	}
	out := &_objectStash{}
	clone._objectStash[in] = out
	return out, false
}

func (clone *_clone) fnStash(in *_fnStash) (*_fnStash, bool) {
	if out, exists := clone._fnStash[in]; exists {
		return out, true
	}
	out := &_fnStash{}
	clone._fnStash[in] = out
	return out, false
}

func (clone *_clone) value(in Value) Value {
	out := in
	switch value := in.value.(type) {
	case *_object:
		out.value = clone.object(value)
	}
	return out
}

func (clone *_clone) valueArray(in []Value) []Value {
	out := make([]Value, len(in))
	for index, value := range in {
		out[index] = clone.value(value)
	}
	return out
}

func (clone *_clone) stash(in _stash) _stash {
	if in == nil {
		return nil
	}
	return in.clone(clone)
}

func (clone *_clone) property(in _property) _property {
	out := in

	switch value := in.value.(type) {
	case Value:
		out.value = clone.value(value)
	case _propertyGetSet:
		p := _propertyGetSet{}
		if value[0] != nil {
			p[0] = clone.object(value[0])
		}
		if value[1] != nil {
			p[1] = clone.object(value[1])
		}
		out.value = p
	default:
		panic(fmt.Errorf("in.value.(Value) != true; in.value is %T", in.value))
	}

	return out
}

func (clone *_clone) dclProperty(in _dclProperty) _dclProperty {
	out := in
	out.value = clone.value(in.value)
	return out
}
