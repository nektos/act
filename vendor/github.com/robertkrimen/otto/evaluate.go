package otto

import (
	"fmt"
	"math"
	"strings"

	"github.com/robertkrimen/otto/token"
)

func (self *_runtime) evaluateMultiply(left float64, right float64) Value {
	// TODO 11.5.1
	return Value{}
}

func (self *_runtime) evaluateDivide(left float64, right float64) Value {
	if math.IsNaN(left) || math.IsNaN(right) {
		return NaNValue()
	}
	if math.IsInf(left, 0) && math.IsInf(right, 0) {
		return NaNValue()
	}
	if left == 0 && right == 0 {
		return NaNValue()
	}
	if math.IsInf(left, 0) {
		if math.Signbit(left) == math.Signbit(right) {
			return positiveInfinityValue()
		} else {
			return negativeInfinityValue()
		}
	}
	if math.IsInf(right, 0) {
		if math.Signbit(left) == math.Signbit(right) {
			return positiveZeroValue()
		} else {
			return negativeZeroValue()
		}
	}
	if right == 0 {
		if math.Signbit(left) == math.Signbit(right) {
			return positiveInfinityValue()
		} else {
			return negativeInfinityValue()
		}
	}
	return toValue_float64(left / right)
}

func (self *_runtime) evaluateModulo(left float64, right float64) Value {
	// TODO 11.5.3
	return Value{}
}

func (self *_runtime) calculateBinaryExpression(operator token.Token, left Value, right Value) Value {

	leftValue := left.resolve()

	switch operator {

	// Additive
	case token.PLUS:
		leftValue = toPrimitive(leftValue)
		rightValue := right.resolve()
		rightValue = toPrimitive(rightValue)

		if leftValue.IsString() || rightValue.IsString() {
			return toValue_string(strings.Join([]string{leftValue.string(), rightValue.string()}, ""))
		} else {
			return toValue_float64(leftValue.float64() + rightValue.float64())
		}
	case token.MINUS:
		rightValue := right.resolve()
		return toValue_float64(leftValue.float64() - rightValue.float64())

		// Multiplicative
	case token.MULTIPLY:
		rightValue := right.resolve()
		return toValue_float64(leftValue.float64() * rightValue.float64())
	case token.SLASH:
		rightValue := right.resolve()
		return self.evaluateDivide(leftValue.float64(), rightValue.float64())
	case token.REMAINDER:
		rightValue := right.resolve()
		return toValue_float64(math.Mod(leftValue.float64(), rightValue.float64()))

		// Logical
	case token.LOGICAL_AND:
		left := leftValue.bool()
		if !left {
			return falseValue
		}
		return toValue_bool(right.resolve().bool())
	case token.LOGICAL_OR:
		left := leftValue.bool()
		if left {
			return trueValue
		}
		return toValue_bool(right.resolve().bool())

		// Bitwise
	case token.AND:
		rightValue := right.resolve()
		return toValue_int32(toInt32(leftValue) & toInt32(rightValue))
	case token.OR:
		rightValue := right.resolve()
		return toValue_int32(toInt32(leftValue) | toInt32(rightValue))
	case token.EXCLUSIVE_OR:
		rightValue := right.resolve()
		return toValue_int32(toInt32(leftValue) ^ toInt32(rightValue))

		// Shift
		// (Masking of 0x1f is to restrict the shift to a maximum of 31 places)
	case token.SHIFT_LEFT:
		rightValue := right.resolve()
		return toValue_int32(toInt32(leftValue) << (toUint32(rightValue) & 0x1f))
	case token.SHIFT_RIGHT:
		rightValue := right.resolve()
		return toValue_int32(toInt32(leftValue) >> (toUint32(rightValue) & 0x1f))
	case token.UNSIGNED_SHIFT_RIGHT:
		rightValue := right.resolve()
		// Shifting an unsigned integer is a logical shift
		return toValue_uint32(toUint32(leftValue) >> (toUint32(rightValue) & 0x1f))

	case token.INSTANCEOF:
		rightValue := right.resolve()
		if !rightValue.IsObject() {
			panic(self.panicTypeError("Expecting a function in instanceof check, but got: %v", rightValue))
		}
		return toValue_bool(rightValue._object().hasInstance(leftValue))

	case token.IN:
		rightValue := right.resolve()
		if !rightValue.IsObject() {
			panic(self.panicTypeError())
		}
		return toValue_bool(rightValue._object().hasProperty(leftValue.string()))
	}

	panic(hereBeDragons(operator))
}

func valueKindDispatchKey(left _valueKind, right _valueKind) int {
	return (int(left) << 2) + int(right)
}

var equalDispatch map[int](func(Value, Value) bool) = makeEqualDispatch()

func makeEqualDispatch() map[int](func(Value, Value) bool) {
	key := valueKindDispatchKey
	return map[int](func(Value, Value) bool){

		key(valueNumber, valueObject): func(x Value, y Value) bool { return x.float64() == y.float64() },
		key(valueString, valueObject): func(x Value, y Value) bool { return x.float64() == y.float64() },
		key(valueObject, valueNumber): func(x Value, y Value) bool { return x.float64() == y.float64() },
		key(valueObject, valueString): func(x Value, y Value) bool { return x.float64() == y.float64() },
	}
}

type _lessThanResult int

const (
	lessThanFalse _lessThanResult = iota
	lessThanTrue
	lessThanUndefined
)

func calculateLessThan(left Value, right Value, leftFirst bool) _lessThanResult {

	x := Value{}
	y := x

	if leftFirst {
		x = toNumberPrimitive(left)
		y = toNumberPrimitive(right)
	} else {
		y = toNumberPrimitive(right)
		x = toNumberPrimitive(left)
	}

	result := false
	if x.kind != valueString || y.kind != valueString {
		x, y := x.float64(), y.float64()
		if math.IsNaN(x) || math.IsNaN(y) {
			return lessThanUndefined
		}
		result = x < y
	} else {
		x, y := x.string(), y.string()
		result = x < y
	}

	if result {
		return lessThanTrue
	}

	return lessThanFalse
}

// FIXME Probably a map is not the most efficient way to do this
var lessThanTable [4](map[_lessThanResult]bool) = [4](map[_lessThanResult]bool){
	// <
	map[_lessThanResult]bool{
		lessThanFalse:     false,
		lessThanTrue:      true,
		lessThanUndefined: false,
	},

	// >
	map[_lessThanResult]bool{
		lessThanFalse:     false,
		lessThanTrue:      true,
		lessThanUndefined: false,
	},

	// <=
	map[_lessThanResult]bool{
		lessThanFalse:     true,
		lessThanTrue:      false,
		lessThanUndefined: false,
	},

	// >=
	map[_lessThanResult]bool{
		lessThanFalse:     true,
		lessThanTrue:      false,
		lessThanUndefined: false,
	},
}

func (self *_runtime) calculateComparison(comparator token.Token, left Value, right Value) bool {

	// FIXME Use strictEqualityComparison?
	// TODO This might be redundant now (with regards to evaluateComparison)
	x := left.resolve()
	y := right.resolve()

	kindEqualKind := false
	result := true
	negate := false

	switch comparator {
	case token.LESS:
		result = lessThanTable[0][calculateLessThan(x, y, true)]
	case token.GREATER:
		result = lessThanTable[1][calculateLessThan(y, x, false)]
	case token.LESS_OR_EQUAL:
		result = lessThanTable[2][calculateLessThan(y, x, false)]
	case token.GREATER_OR_EQUAL:
		result = lessThanTable[3][calculateLessThan(x, y, true)]
	case token.STRICT_NOT_EQUAL:
		negate = true
		fallthrough
	case token.STRICT_EQUAL:
		if x.kind != y.kind {
			result = false
		} else {
			kindEqualKind = true
		}
	case token.NOT_EQUAL:
		negate = true
		fallthrough
	case token.EQUAL:
		if x.kind == y.kind {
			kindEqualKind = true
		} else if x.kind <= valueNull && y.kind <= valueNull {
			result = true
		} else if x.kind <= valueNull || y.kind <= valueNull {
			result = false
		} else if x.kind <= valueString && y.kind <= valueString {
			result = x.float64() == y.float64()
		} else if x.kind == valueBoolean {
			result = self.calculateComparison(token.EQUAL, toValue_float64(x.float64()), y)
		} else if y.kind == valueBoolean {
			result = self.calculateComparison(token.EQUAL, x, toValue_float64(y.float64()))
		} else if x.kind == valueObject {
			result = self.calculateComparison(token.EQUAL, toPrimitive(x), y)
		} else if y.kind == valueObject {
			result = self.calculateComparison(token.EQUAL, x, toPrimitive(y))
		} else {
			panic(hereBeDragons("Unable to test for equality: %v ==? %v", x, y))
		}
	default:
		panic(fmt.Errorf("Unknown comparator %s", comparator.String()))
	}

	if kindEqualKind {
		switch x.kind {
		case valueUndefined, valueNull:
			result = true
		case valueNumber:
			x := x.float64()
			y := y.float64()
			if math.IsNaN(x) || math.IsNaN(y) {
				result = false
			} else {
				result = x == y
			}
		case valueString:
			result = x.string() == y.string()
		case valueBoolean:
			result = x.bool() == y.bool()
		case valueObject:
			result = x._object() == y._object()
		default:
			goto ERROR
		}
	}

	if negate {
		result = !result
	}

	return result

ERROR:
	panic(hereBeDragons("%v (%v) %s %v (%v)", x, x.kind, comparator, y, y.kind))
}
