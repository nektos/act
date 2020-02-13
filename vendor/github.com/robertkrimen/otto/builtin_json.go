package otto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type _builtinJSON_parseContext struct {
	call    FunctionCall
	reviver Value
}

func builtinJSON_parse(call FunctionCall) Value {
	ctx := _builtinJSON_parseContext{
		call: call,
	}
	revive := false
	if reviver := call.Argument(1); reviver.isCallable() {
		revive = true
		ctx.reviver = reviver
	}

	var root interface{}
	err := json.Unmarshal([]byte(call.Argument(0).string()), &root)
	if err != nil {
		panic(call.runtime.panicSyntaxError(err.Error()))
	}
	value, exists := builtinJSON_parseWalk(ctx, root)
	if !exists {
		value = Value{}
	}
	if revive {
		root := ctx.call.runtime.newObject()
		root.put("", value, false)
		return builtinJSON_reviveWalk(ctx, root, "")
	}
	return value
}

func builtinJSON_reviveWalk(ctx _builtinJSON_parseContext, holder *_object, name string) Value {
	value := holder.get(name)
	if object := value._object(); object != nil {
		if isArray(object) {
			length := int64(objectLength(object))
			for index := int64(0); index < length; index += 1 {
				name := arrayIndexToString(index)
				value := builtinJSON_reviveWalk(ctx, object, name)
				if value.IsUndefined() {
					object.delete(name, false)
				} else {
					object.defineProperty(name, value, 0111, false)
				}
			}
		} else {
			object.enumerate(false, func(name string) bool {
				value := builtinJSON_reviveWalk(ctx, object, name)
				if value.IsUndefined() {
					object.delete(name, false)
				} else {
					object.defineProperty(name, value, 0111, false)
				}
				return true
			})
		}
	}
	return ctx.reviver.call(ctx.call.runtime, toValue_object(holder), name, value)
}

func builtinJSON_parseWalk(ctx _builtinJSON_parseContext, rawValue interface{}) (Value, bool) {
	switch value := rawValue.(type) {
	case nil:
		return nullValue, true
	case bool:
		return toValue_bool(value), true
	case string:
		return toValue_string(value), true
	case float64:
		return toValue_float64(value), true
	case []interface{}:
		arrayValue := make([]Value, len(value))
		for index, rawValue := range value {
			if value, exists := builtinJSON_parseWalk(ctx, rawValue); exists {
				arrayValue[index] = value
			}
		}
		return toValue_object(ctx.call.runtime.newArrayOf(arrayValue)), true
	case map[string]interface{}:
		object := ctx.call.runtime.newObject()
		for name, rawValue := range value {
			if value, exists := builtinJSON_parseWalk(ctx, rawValue); exists {
				object.put(name, value, false)
			}
		}
		return toValue_object(object), true
	}
	return Value{}, false
}

type _builtinJSON_stringifyContext struct {
	call             FunctionCall
	stack            []*_object
	propertyList     []string
	replacerFunction *Value
	gap              string
}

func builtinJSON_stringify(call FunctionCall) Value {
	ctx := _builtinJSON_stringifyContext{
		call:  call,
		stack: []*_object{nil},
	}
	replacer := call.Argument(1)._object()
	if replacer != nil {
		if isArray(replacer) {
			length := objectLength(replacer)
			seen := map[string]bool{}
			propertyList := make([]string, length)
			length = 0
			for index, _ := range propertyList {
				value := replacer.get(arrayIndexToString(int64(index)))
				switch value.kind {
				case valueObject:
					switch value.value.(*_object).class {
					case "String":
					case "Number":
					default:
						continue
					}
				case valueString:
				case valueNumber:
				default:
					continue
				}
				name := value.string()
				if seen[name] {
					continue
				}
				seen[name] = true
				length += 1
				propertyList[index] = name
			}
			ctx.propertyList = propertyList[0:length]
		} else if replacer.class == "Function" {
			value := toValue_object(replacer)
			ctx.replacerFunction = &value
		}
	}
	if spaceValue, exists := call.getArgument(2); exists {
		if spaceValue.kind == valueObject {
			switch spaceValue.value.(*_object).class {
			case "String":
				spaceValue = toValue_string(spaceValue.string())
			case "Number":
				spaceValue = spaceValue.numberValue()
			}
		}
		switch spaceValue.kind {
		case valueString:
			value := spaceValue.string()
			if len(value) > 10 {
				ctx.gap = value[0:10]
			} else {
				ctx.gap = value
			}
		case valueNumber:
			value := spaceValue.number().int64
			if value > 10 {
				value = 10
			} else if value < 0 {
				value = 0
			}
			ctx.gap = strings.Repeat(" ", int(value))
		}
	}
	holder := call.runtime.newObject()
	holder.put("", call.Argument(0), false)
	value, exists := builtinJSON_stringifyWalk(ctx, "", holder)
	if !exists {
		return Value{}
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		panic(call.runtime.panicTypeError(err.Error()))
	}
	if ctx.gap != "" {
		valueJSON1 := bytes.Buffer{}
		json.Indent(&valueJSON1, valueJSON, "", ctx.gap)
		valueJSON = valueJSON1.Bytes()
	}
	return toValue_string(string(valueJSON))
}

func builtinJSON_stringifyWalk(ctx _builtinJSON_stringifyContext, key string, holder *_object) (interface{}, bool) {
	value := holder.get(key)

	if value.IsObject() {
		object := value._object()
		if toJSON := object.get("toJSON"); toJSON.IsFunction() {
			value = toJSON.call(ctx.call.runtime, value, key)
		} else {
			// If the object is a GoStruct or something that implements json.Marshaler
			if object.objectClass.marshalJSON != nil {
				marshaler := object.objectClass.marshalJSON(object)
				if marshaler != nil {
					return marshaler, true
				}
			}
		}
	}

	if ctx.replacerFunction != nil {
		value = (*ctx.replacerFunction).call(ctx.call.runtime, toValue_object(holder), key, value)
	}

	if value.kind == valueObject {
		switch value.value.(*_object).class {
		case "Boolean":
			value = value._object().value.(Value)
		case "String":
			value = toValue_string(value.string())
		case "Number":
			value = value.numberValue()
		}
	}

	switch value.kind {
	case valueBoolean:
		return value.bool(), true
	case valueString:
		return value.string(), true
	case valueNumber:
		integer := value.number()
		switch integer.kind {
		case numberInteger:
			return integer.int64, true
		case numberFloat:
			return integer.float64, true
		default:
			return nil, true
		}
	case valueNull:
		return nil, true
	case valueObject:
		holder := value._object()
		if value := value._object(); nil != value {
			for _, object := range ctx.stack {
				if holder == object {
					panic(ctx.call.runtime.panicTypeError("Converting circular structure to JSON"))
				}
			}
			ctx.stack = append(ctx.stack, value)
			defer func() { ctx.stack = ctx.stack[:len(ctx.stack)-1] }()
		}
		if isArray(holder) {
			var length uint32
			switch value := holder.get("length").value.(type) {
			case uint32:
				length = value
			case int:
				if value >= 0 {
					length = uint32(value)
				}
			default:
				panic(ctx.call.runtime.panicTypeError(fmt.Sprintf("JSON.stringify: invalid length: %v (%[1]T)", value)))
			}
			array := make([]interface{}, length)
			for index, _ := range array {
				name := arrayIndexToString(int64(index))
				value, _ := builtinJSON_stringifyWalk(ctx, name, holder)
				array[index] = value
			}
			return array, true
		} else if holder.class != "Function" {
			object := map[string]interface{}{}
			if ctx.propertyList != nil {
				for _, name := range ctx.propertyList {
					value, exists := builtinJSON_stringifyWalk(ctx, name, holder)
					if exists {
						object[name] = value
					}
				}
			} else {
				// Go maps are without order, so this doesn't conform to the ECMA ordering
				// standard, but oh well...
				holder.enumerate(false, func(name string) bool {
					value, exists := builtinJSON_stringifyWalk(ctx, name, holder)
					if exists {
						object[name] = value
					}
					return true
				})
			}
			return object, true
		}
	}
	return nil, false
}
