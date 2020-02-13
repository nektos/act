package otto

import (
	"strconv"
)

func (runtime *_runtime) newArrayObject(length uint32) *_object {
	self := runtime.newObject()
	self.class = "Array"
	self.defineProperty("length", toValue_uint32(length), 0100, false)
	self.objectClass = _classArray
	return self
}

func isArray(object *_object) bool {
	return object != nil && (object.class == "Array" || object.class == "GoArray")
}

func objectLength(object *_object) uint32 {
	if object == nil {
		return 0
	}
	switch object.class {
	case "Array":
		return object.get("length").value.(uint32)
	case "String":
		return uint32(object.get("length").value.(int))
	case "GoArray":
		return uint32(object.get("length").value.(int))
	}
	return 0
}

func arrayUint32(rt *_runtime, value Value) uint32 {
	nm := value.number()
	if nm.kind != numberInteger || !isUint32(nm.int64) {
		// FIXME
		panic(rt.panicRangeError())
	}
	return uint32(nm.int64)
}

func arrayDefineOwnProperty(self *_object, name string, descriptor _property, throw bool) bool {
	lengthProperty := self.getOwnProperty("length")
	lengthValue, valid := lengthProperty.value.(Value)
	if !valid {
		panic("Array.length != Value{}")
	}
	length := lengthValue.value.(uint32)
	if name == "length" {
		if descriptor.value == nil {
			return objectDefineOwnProperty(self, name, descriptor, throw)
		}
		newLengthValue, isValue := descriptor.value.(Value)
		if !isValue {
			panic(self.runtime.panicTypeError())
		}
		newLength := arrayUint32(self.runtime, newLengthValue)
		descriptor.value = toValue_uint32(newLength)
		if newLength > length {
			return objectDefineOwnProperty(self, name, descriptor, throw)
		}
		if !lengthProperty.writable() {
			goto Reject
		}
		newWritable := true
		if descriptor.mode&0700 == 0 {
			// If writable is off
			newWritable = false
			descriptor.mode |= 0100
		}
		if !objectDefineOwnProperty(self, name, descriptor, throw) {
			return false
		}
		for newLength < length {
			length--
			if !self.delete(strconv.FormatInt(int64(length), 10), false) {
				descriptor.value = toValue_uint32(length + 1)
				if !newWritable {
					descriptor.mode &= 0077
				}
				objectDefineOwnProperty(self, name, descriptor, false)
				goto Reject
			}
		}
		if !newWritable {
			descriptor.mode &= 0077
			objectDefineOwnProperty(self, name, descriptor, false)
		}
	} else if index := stringToArrayIndex(name); index >= 0 {
		if index >= int64(length) && !lengthProperty.writable() {
			goto Reject
		}
		if !objectDefineOwnProperty(self, strconv.FormatInt(index, 10), descriptor, false) {
			goto Reject
		}
		if index >= int64(length) {
			lengthProperty.value = toValue_uint32(uint32(index + 1))
			objectDefineOwnProperty(self, "length", *lengthProperty, false)
			return true
		}
	}
	return objectDefineOwnProperty(self, name, descriptor, throw)
Reject:
	if throw {
		panic(self.runtime.panicTypeError())
	}
	return false
}
