package otto

func (runtime *_runtime) newNumberObject(value Value) *_object {
	return runtime.newPrimitiveObject("Number", value.numberValue())
}
