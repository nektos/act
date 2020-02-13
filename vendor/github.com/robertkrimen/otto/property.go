package otto

// property

type _propertyMode int

const (
	modeWriteMask     _propertyMode = 0700
	modeEnumerateMask               = 0070
	modeConfigureMask               = 0007
	modeOnMask                      = 0111
	modeOffMask                     = 0000
	modeSetMask                     = 0222 // If value is 2, then mode is neither "On" nor "Off"
)

type _propertyGetSet [2]*_object

var _nilGetSetObject _object = _object{}

type _property struct {
	value interface{}
	mode  _propertyMode
}

func (self _property) writable() bool {
	return self.mode&modeWriteMask == modeWriteMask&modeOnMask
}

func (self *_property) writeOn() {
	self.mode = (self.mode & ^modeWriteMask) | (modeWriteMask & modeOnMask)
}

func (self *_property) writeOff() {
	self.mode &= ^modeWriteMask
}

func (self *_property) writeClear() {
	self.mode = (self.mode & ^modeWriteMask) | (modeWriteMask & modeSetMask)
}

func (self _property) writeSet() bool {
	return 0 == self.mode&modeWriteMask&modeSetMask
}

func (self _property) enumerable() bool {
	return self.mode&modeEnumerateMask == modeEnumerateMask&modeOnMask
}

func (self *_property) enumerateOn() {
	self.mode = (self.mode & ^modeEnumerateMask) | (modeEnumerateMask & modeOnMask)
}

func (self *_property) enumerateOff() {
	self.mode &= ^modeEnumerateMask
}

func (self _property) enumerateSet() bool {
	return 0 == self.mode&modeEnumerateMask&modeSetMask
}

func (self _property) configurable() bool {
	return self.mode&modeConfigureMask == modeConfigureMask&modeOnMask
}

func (self *_property) configureOn() {
	self.mode = (self.mode & ^modeConfigureMask) | (modeConfigureMask & modeOnMask)
}

func (self *_property) configureOff() {
	self.mode &= ^modeConfigureMask
}

func (self _property) configureSet() bool {
	return 0 == self.mode&modeConfigureMask&modeSetMask
}

func (self _property) copy() *_property {
	property := self
	return &property
}

func (self _property) get(this *_object) Value {
	switch value := self.value.(type) {
	case Value:
		return value
	case _propertyGetSet:
		if value[0] != nil {
			return value[0].call(toValue(this), nil, false, nativeFrame)
		}
	}
	return Value{}
}

func (self _property) isAccessorDescriptor() bool {
	setGet, test := self.value.(_propertyGetSet)
	return test && (setGet[0] != nil || setGet[1] != nil)
}

func (self _property) isDataDescriptor() bool {
	if self.writeSet() { // Either "On" or "Off"
		return true
	}
	value, valid := self.value.(Value)
	return valid && !value.isEmpty()
}

func (self _property) isGenericDescriptor() bool {
	return !(self.isDataDescriptor() || self.isAccessorDescriptor())
}

func (self _property) isEmpty() bool {
	return self.mode == 0222 && self.isGenericDescriptor()
}

// _enumerableValue, _enumerableTrue, _enumerableFalse?
// .enumerableValue() .enumerableExists()

func toPropertyDescriptor(rt *_runtime, value Value) (descriptor _property) {
	objectDescriptor := value._object()
	if objectDescriptor == nil {
		panic(rt.panicTypeError())
	}

	{
		descriptor.mode = modeSetMask // Initially nothing is set
		if objectDescriptor.hasProperty("enumerable") {
			if objectDescriptor.get("enumerable").bool() {
				descriptor.enumerateOn()
			} else {
				descriptor.enumerateOff()
			}
		}

		if objectDescriptor.hasProperty("configurable") {
			if objectDescriptor.get("configurable").bool() {
				descriptor.configureOn()
			} else {
				descriptor.configureOff()
			}
		}

		if objectDescriptor.hasProperty("writable") {
			if objectDescriptor.get("writable").bool() {
				descriptor.writeOn()
			} else {
				descriptor.writeOff()
			}
		}
	}

	var getter, setter *_object
	getterSetter := false

	if objectDescriptor.hasProperty("get") {
		value := objectDescriptor.get("get")
		if value.IsDefined() {
			if !value.isCallable() {
				panic(rt.panicTypeError())
			}
			getter = value._object()
			getterSetter = true
		} else {
			getter = &_nilGetSetObject
			getterSetter = true
		}
	}

	if objectDescriptor.hasProperty("set") {
		value := objectDescriptor.get("set")
		if value.IsDefined() {
			if !value.isCallable() {
				panic(rt.panicTypeError())
			}
			setter = value._object()
			getterSetter = true
		} else {
			setter = &_nilGetSetObject
			getterSetter = true
		}
	}

	if getterSetter {
		if descriptor.writeSet() {
			panic(rt.panicTypeError())
		}
		descriptor.value = _propertyGetSet{getter, setter}
	}

	if objectDescriptor.hasProperty("value") {
		if getterSetter {
			panic(rt.panicTypeError())
		}
		descriptor.value = objectDescriptor.get("value")
	}

	return
}

func (self *_runtime) fromPropertyDescriptor(descriptor _property) *_object {
	object := self.newObject()
	if descriptor.isDataDescriptor() {
		object.defineProperty("value", descriptor.value.(Value), 0111, false)
		object.defineProperty("writable", toValue_bool(descriptor.writable()), 0111, false)
	} else if descriptor.isAccessorDescriptor() {
		getSet := descriptor.value.(_propertyGetSet)
		get := Value{}
		if getSet[0] != nil {
			get = toValue_object(getSet[0])
		}
		set := Value{}
		if getSet[1] != nil {
			set = toValue_object(getSet[1])
		}
		object.defineProperty("get", get, 0111, false)
		object.defineProperty("set", set, 0111, false)
	}
	object.defineProperty("enumerable", toValue_bool(descriptor.enumerable()), 0111, false)
	object.defineProperty("configurable", toValue_bool(descriptor.configurable()), 0111, false)
	return object
}
