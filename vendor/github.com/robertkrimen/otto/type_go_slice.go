package otto

import (
	"reflect"
	"strconv"
)

func (runtime *_runtime) newGoSliceObject(value reflect.Value) *_object {
	self := runtime.newObject()
	self.class = "GoArray" // TODO GoSlice?
	self.objectClass = _classGoSlice
	self.value = _newGoSliceObject(value)
	return self
}

type _goSliceObject struct {
	value reflect.Value
}

func _newGoSliceObject(value reflect.Value) *_goSliceObject {
	self := &_goSliceObject{
		value: value,
	}
	return self
}

func (self _goSliceObject) getValue(index int64) (reflect.Value, bool) {
	if index < int64(self.value.Len()) {
		return self.value.Index(int(index)), true
	}
	return reflect.Value{}, false
}

func (self _goSliceObject) setValue(index int64, value Value) bool {
	indexValue, exists := self.getValue(index)
	if !exists {
		return false
	}
	reflectValue, err := value.toReflectValue(self.value.Type().Elem().Kind())
	if err != nil {
		panic(err)
	}
	indexValue.Set(reflectValue)
	return true
}

func goSliceGetOwnProperty(self *_object, name string) *_property {
	// length
	if name == "length" {
		return &_property{
			value: toValue(self.value.(*_goSliceObject).value.Len()),
			mode:  0,
		}
	}

	// .0, .1, .2, ...
	index := stringToArrayIndex(name)
	if index >= 0 {
		value := Value{}
		reflectValue, exists := self.value.(*_goSliceObject).getValue(index)
		if exists {
			value = self.runtime.toValue(reflectValue.Interface())
		}
		return &_property{
			value: value,
			mode:  0110,
		}
	}

	// Other methods
	if method := self.value.(*_goSliceObject).value.MethodByName(name); (method != reflect.Value{}) {
		return &_property{
			value: self.runtime.toValue(method.Interface()),
			mode:  0110,
		}
	}

	return objectGetOwnProperty(self, name)
}

func goSliceEnumerate(self *_object, all bool, each func(string) bool) {
	object := self.value.(*_goSliceObject)
	// .0, .1, .2, ...

	for index, length := 0, object.value.Len(); index < length; index++ {
		name := strconv.FormatInt(int64(index), 10)
		if !each(name) {
			return
		}
	}

	objectEnumerate(self, all, each)
}

func goSliceDefineOwnProperty(self *_object, name string, descriptor _property, throw bool) bool {
	if name == "length" {
		return self.runtime.typeErrorResult(throw)
	} else if index := stringToArrayIndex(name); index >= 0 {
		if self.value.(*_goSliceObject).setValue(index, descriptor.value.(Value)) {
			return true
		}
		return self.runtime.typeErrorResult(throw)
	}
	return objectDefineOwnProperty(self, name, descriptor, throw)
}

func goSliceDelete(self *_object, name string, throw bool) bool {
	// length
	if name == "length" {
		return self.runtime.typeErrorResult(throw)
	}

	// .0, .1, .2, ...
	index := stringToArrayIndex(name)
	if index >= 0 {
		object := self.value.(*_goSliceObject)
		indexValue, exists := object.getValue(index)
		if exists {
			indexValue.Set(reflect.Zero(object.value.Type().Elem()))
			return true
		}
		return self.runtime.typeErrorResult(throw)
	}

	return self.delete(name, throw)
}
