package otto

import (
	"strconv"
)

func (runtime *_runtime) newBooleanObject(value Value) *_object {
	return runtime.newPrimitiveObject("Boolean", toValue_bool(value.bool()))
}

func booleanToString(value bool) string {
	return strconv.FormatBool(value)
}
