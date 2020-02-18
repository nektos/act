package otto

import (
	"fmt"
	"math"
	"reflect"
	"unicode/utf16"
)

func (value Value) bool() bool {
	if value.kind == valueBoolean {
		return value.value.(bool)
	}
	if value.IsUndefined() {
		return false
	}
	if value.IsNull() {
		return false
	}
	switch value := value.value.(type) {
	case bool:
		return value
	case int, int8, int16, int32, int64:
		return 0 != reflect.ValueOf(value).Int()
	case uint, uint8, uint16, uint32, uint64:
		return 0 != reflect.ValueOf(value).Uint()
	case float32:
		return 0 != value
	case float64:
		if math.IsNaN(value) || value == 0 {
			return false
		}
		return true
	case string:
		return 0 != len(value)
	case []uint16:
		return 0 != len(utf16.Decode(value))
	}
	if value.IsObject() {
		return true
	}
	panic(fmt.Errorf("toBoolean(%T)", value.value))
}
