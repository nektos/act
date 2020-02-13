package otto

import (
	"reflect"
	"strconv"
)

func (runtime *_runtime) newGoArrayObject(value reflect.Value) *_object {
	self := runtime.newObject()
	self.class = "GoArray"
	self.objectClass = _classGoArray
	self.value = _newGoArrayObject(value)
	return self
}

type _goArrayObject struct {
	value        reflect.Value
	writable     bool
	propertyMode _propertyMode
}

func _newGoArrayObject(value reflect.Value) *_goArrayObject {
	writable := value.Kind() == reflect.Ptr // The Array is addressable (like a Slice)
	mode := _propertyMode(0010)
	if writable {
		mode = 0110
	}
	self := &_goArrayObject{
		value:        value,
		writable:     writable,
		propertyMode: mode,
	}
	return self
}

func (self _goArrayObject) getValue(name string) (reflect.Value, bool) {
	if index, err := strconv.ParseInt(name, 10, 64); err != nil {
		v, ok := self.getValueIndex(index)
		if ok {
			return v, ok
		}
	}

	if m := self.value.MethodByName(name); m != (reflect.Value{}) {
		return m, true
	}

	return reflect.Value{}, false
}

func (self _goArrayObject) getValueIndex(index int64) (reflect.Value, bool) {
	value := reflect.Indirect(self.value)
	if index < int64(value.Len()) {
		return value.Index(int(index)), true
	}

	return reflect.Value{}, false
}

func (self _goArrayObject) setValue(index int64, value Value) bool {
	indexValue, exists := self.getValueIndex(index)
	if !exists {
		return false
	}
	reflectValue, err := value.toReflectValue(reflect.Indirect(self.value).Type().Elem().Kind())
	if err != nil {
		panic(err)
	}
	indexValue.Set(reflectValue)
	return true
}

func goArrayGetOwnProperty(self *_object, name string) *_property {
	// length
	if name == "length" {
		return &_property{
			value: toValue(reflect.Indirect(self.value.(*_goArrayObject).value).Len()),
			mode:  0,
		}
	}

	// .0, .1, .2, ...
	if index := stringToArrayIndex(name); index >= 0 {
		object := self.value.(*_goArrayObject)
		value := Value{}
		reflectValue, exists := object.getValueIndex(index)
		if exists {
			value = self.runtime.toValue(reflectValue.Interface())
		}
		return &_property{
			value: value,
			mode:  object.propertyMode,
		}
	}

	if method := self.value.(*_goArrayObject).value.MethodByName(name); method != (reflect.Value{}) {
		return &_property{
			self.runtime.toValue(method.Interface()),
			0110,
		}
	}

	return objectGetOwnProperty(self, name)
}

func goArrayEnumerate(self *_object, all bool, each func(string) bool) {
	object := self.value.(*_goArrayObject)
	// .0, .1, .2, ...

	for index, length := 0, object.value.Len(); index < length; index++ {
		name := strconv.FormatInt(int64(index), 10)
		if !each(name) {
			return
		}
	}

	objectEnumerate(self, all, each)
}

func goArrayDefineOwnProperty(self *_object, name string, descriptor _property, throw bool) bool {
	if name == "length" {
		return self.runtime.typeErrorResult(throw)
	} else if index := stringToArrayIndex(name); index >= 0 {
		object := self.value.(*_goArrayObject)
		if object.writable {
			if self.value.(*_goArrayObject).setValue(index, descriptor.value.(Value)) {
				return true
			}
		}
		return self.runtime.typeErrorResult(throw)
	}
	return objectDefineOwnProperty(self, name, descriptor, throw)
}

func goArrayDelete(self *_object, name string, throw bool) bool {
	// length
	if name == "length" {
		return self.runtime.typeErrorResult(throw)
	}

	// .0, .1, .2, ...
	index := stringToArrayIndex(name)
	if index >= 0 {
		object := self.value.(*_goArrayObject)
		if object.writable {
			indexValue, exists := object.getValueIndex(index)
			if exists {
				indexValue.Set(reflect.Zero(reflect.Indirect(object.value).Type().Elem()))
				return true
			}
		}
		return self.runtime.typeErrorResult(throw)
	}

	return self.delete(name, throw)
}
