package otto

import (
	"reflect"
)

func (runtime *_runtime) newGoMapObject(value reflect.Value) *_object {
	self := runtime.newObject()
	self.class = "Object" // TODO Should this be something else?
	self.objectClass = _classGoMap
	self.value = _newGoMapObject(value)
	return self
}

type _goMapObject struct {
	value     reflect.Value
	keyKind   reflect.Kind
	valueKind reflect.Kind
}

func _newGoMapObject(value reflect.Value) *_goMapObject {
	if value.Kind() != reflect.Map {
		dbgf("%/panic//%@: %v != reflect.Map", value.Kind())
	}
	self := &_goMapObject{
		value:     value,
		keyKind:   value.Type().Key().Kind(),
		valueKind: value.Type().Elem().Kind(),
	}
	return self
}

func (self _goMapObject) toKey(name string) reflect.Value {
	reflectValue, err := stringToReflectValue(name, self.keyKind)
	if err != nil {
		panic(err)
	}
	return reflectValue
}

func (self _goMapObject) toValue(value Value) reflect.Value {
	reflectValue, err := value.toReflectValue(self.valueKind)
	if err != nil {
		panic(err)
	}
	return reflectValue
}

func goMapGetOwnProperty(self *_object, name string) *_property {
	object := self.value.(*_goMapObject)
	value := object.value.MapIndex(object.toKey(name))
	if value.IsValid() {
		return &_property{self.runtime.toValue(value.Interface()), 0111}
	}

	// Other methods
	if method := self.value.(*_goMapObject).value.MethodByName(name); (method != reflect.Value{}) {
		return &_property{
			value: self.runtime.toValue(method.Interface()),
			mode:  0110,
		}
	}

	return nil
}

func goMapEnumerate(self *_object, all bool, each func(string) bool) {
	object := self.value.(*_goMapObject)
	keys := object.value.MapKeys()
	for _, key := range keys {
		if !each(toValue(key).String()) {
			return
		}
	}
}

func goMapDefineOwnProperty(self *_object, name string, descriptor _property, throw bool) bool {
	object := self.value.(*_goMapObject)
	// TODO ...or 0222
	if descriptor.mode != 0111 {
		return self.runtime.typeErrorResult(throw)
	}
	if !descriptor.isDataDescriptor() {
		return self.runtime.typeErrorResult(throw)
	}
	object.value.SetMapIndex(object.toKey(name), object.toValue(descriptor.value.(Value)))
	return true
}

func goMapDelete(self *_object, name string, throw bool) bool {
	object := self.value.(*_goMapObject)
	object.value.SetMapIndex(object.toKey(name), reflect.Value{})
	// FIXME
	return true
}
