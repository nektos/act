package otto

type _reference interface {
	invalid() bool         // IsUnresolvableReference
	getValue() Value       // getValue
	putValue(Value) string // PutValue
	delete() bool
}

// PropertyReference

type _propertyReference struct {
	name    string
	strict  bool
	base    *_object
	runtime *_runtime
	at      _at
}

func newPropertyReference(rt *_runtime, base *_object, name string, strict bool, at _at) *_propertyReference {
	return &_propertyReference{
		runtime: rt,
		name:    name,
		strict:  strict,
		base:    base,
		at:      at,
	}
}

func (self *_propertyReference) invalid() bool {
	return self.base == nil
}

func (self *_propertyReference) getValue() Value {
	if self.base == nil {
		panic(self.runtime.panicReferenceError("'%s' is not defined", self.name, self.at))
	}
	return self.base.get(self.name)
}

func (self *_propertyReference) putValue(value Value) string {
	if self.base == nil {
		return self.name
	}
	self.base.put(self.name, value, self.strict)
	return ""
}

func (self *_propertyReference) delete() bool {
	if self.base == nil {
		// TODO Throw an error if strict
		return true
	}
	return self.base.delete(self.name, self.strict)
}

// ArgumentReference

func newArgumentReference(runtime *_runtime, base *_object, name string, strict bool, at _at) *_propertyReference {
	if base == nil {
		panic(hereBeDragons())
	}
	return newPropertyReference(runtime, base, name, strict, at)
}

type _stashReference struct {
	name   string
	strict bool
	base   _stash
}

func (self *_stashReference) invalid() bool {
	return false // The base (an environment) will never be nil
}

func (self *_stashReference) getValue() Value {
	return self.base.getBinding(self.name, self.strict)
}

func (self *_stashReference) putValue(value Value) string {
	self.base.setValue(self.name, value, self.strict)
	return ""
}

func (self *_stashReference) delete() bool {
	if self.base == nil {
		// This should never be reached, but just in case
		return false
	}
	return self.base.deleteBinding(self.name)
}

// getIdentifierReference

func getIdentifierReference(runtime *_runtime, stash _stash, name string, strict bool, at _at) _reference {
	if stash == nil {
		return newPropertyReference(runtime, nil, name, strict, at)
	}
	if stash.hasBinding(name) {
		return stash.newReference(name, strict, at)
	}
	return getIdentifierReference(runtime, stash.outer(), name, strict, at)
}
