package otto

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var stringToNumberParseInteger = regexp.MustCompile(`^(?:0[xX])`)

func parseNumber(value string) float64 {
	value = strings.Trim(value, builtinString_trim_whitespace)

	if value == "" {
		return 0
	}

	parseFloat := false
	if strings.IndexRune(value, '.') != -1 {
		parseFloat = true
	} else if stringToNumberParseInteger.MatchString(value) {
		parseFloat = false
	} else {
		parseFloat = true
	}

	if parseFloat {
		number, err := strconv.ParseFloat(value, 64)
		if err != nil && err.(*strconv.NumError).Err != strconv.ErrRange {
			return math.NaN()
		}
		return number
	}

	number, err := strconv.ParseInt(value, 0, 64)
	if err != nil {
		return math.NaN()
	}
	return float64(number)
}

func (value Value) float64() float64 {
	switch value.kind {
	case valueUndefined:
		return math.NaN()
	case valueNull:
		return 0
	}
	switch value := value.value.(type) {
	case bool:
		if value {
			return 1
		}
		return 0
	case int:
		return float64(value)
	case int8:
		return float64(value)
	case int16:
		return float64(value)
	case int32:
		return float64(value)
	case int64:
		return float64(value)
	case uint:
		return float64(value)
	case uint8:
		return float64(value)
	case uint16:
		return float64(value)
	case uint32:
		return float64(value)
	case uint64:
		return float64(value)
	case float64:
		return value
	case string:
		return parseNumber(value)
	case *_object:
		return value.DefaultValue(defaultValueHintNumber).float64()
	}
	panic(fmt.Errorf("toFloat(%T)", value.value))
}

const (
	float_2_64   float64 = 18446744073709551616.0
	float_2_63   float64 = 9223372036854775808.0
	float_2_32   float64 = 4294967296.0
	float_2_31   float64 = 2147483648.0
	float_2_16   float64 = 65536.0
	integer_2_32 int64   = 4294967296
	integer_2_31 int64   = 2146483648
	sqrt1_2      float64 = math.Sqrt2 / 2
)

const (
	maxInt8   = math.MaxInt8
	minInt8   = math.MinInt8
	maxInt16  = math.MaxInt16
	minInt16  = math.MinInt16
	maxInt32  = math.MaxInt32
	minInt32  = math.MinInt32
	maxInt64  = math.MaxInt64
	minInt64  = math.MinInt64
	maxUint8  = math.MaxUint8
	maxUint16 = math.MaxUint16
	maxUint32 = math.MaxUint32
	maxUint64 = math.MaxUint64
	maxUint   = ^uint(0)
	minUint   = 0
	maxInt    = int(^uint(0) >> 1)
	minInt    = -maxInt - 1

	// int64
	int64_maxInt    int64 = int64(maxInt)
	int64_minInt    int64 = int64(minInt)
	int64_maxInt8   int64 = math.MaxInt8
	int64_minInt8   int64 = math.MinInt8
	int64_maxInt16  int64 = math.MaxInt16
	int64_minInt16  int64 = math.MinInt16
	int64_maxInt32  int64 = math.MaxInt32
	int64_minInt32  int64 = math.MinInt32
	int64_maxUint8  int64 = math.MaxUint8
	int64_maxUint16 int64 = math.MaxUint16
	int64_maxUint32 int64 = math.MaxUint32

	// float64
	float_maxInt    float64 = float64(int(^uint(0) >> 1))
	float_minInt    float64 = float64(int(-maxInt - 1))
	float_minUint   float64 = float64(0)
	float_maxUint   float64 = float64(uint(^uint(0)))
	float_minUint64 float64 = float64(0)
	float_maxUint64 float64 = math.MaxUint64
	float_maxInt64  float64 = math.MaxInt64
	float_minInt64  float64 = math.MinInt64
)

func toIntegerFloat(value Value) float64 {
	float := value.float64()
	if math.IsInf(float, 0) {
	} else if math.IsNaN(float) {
		float = 0
	} else if float > 0 {
		float = math.Floor(float)
	} else {
		float = math.Ceil(float)
	}
	return float
}

type _numberKind int

const (
	numberInteger  _numberKind = iota // 3.0 => 3.0
	numberFloat                       // 3.14159 => 3.0, 1+2**63 > 2**63-1
	numberInfinity                    // Infinity => 2**63-1
	numberNaN                         // NaN => 0
)

type _number struct {
	kind    _numberKind
	int64   int64
	float64 float64
}

// FIXME
// http://www.goinggo.net/2013/08/gustavos-ieee-754-brain-teaser.html
// http://bazaar.launchpad.net/~niemeyer/strepr/trunk/view/6/strepr.go#L160
func (value Value) number() (number _number) {
	switch value := value.value.(type) {
	case int8:
		number.int64 = int64(value)
		return
	case int16:
		number.int64 = int64(value)
		return
	case uint8:
		number.int64 = int64(value)
		return
	case uint16:
		number.int64 = int64(value)
		return
	case uint32:
		number.int64 = int64(value)
		return
	case int:
		number.int64 = int64(value)
		return
	case int64:
		number.int64 = value
		return
	}

	float := value.float64()
	if float == 0 {
		return
	}

	number.kind = numberFloat
	number.float64 = float

	if math.IsNaN(float) {
		number.kind = numberNaN
		return
	}

	if math.IsInf(float, 0) {
		number.kind = numberInfinity
	}

	if float >= float_maxInt64 {
		number.int64 = math.MaxInt64
		return
	}

	if float <= float_minInt64 {
		number.int64 = math.MinInt64
		return
	}

	integer := float64(0)
	if float > 0 {
		integer = math.Floor(float)
	} else {
		integer = math.Ceil(float)
	}

	if float == integer {
		number.kind = numberInteger
	}
	number.int64 = int64(float)
	return
}

// ECMA 262: 9.5
func toInt32(value Value) int32 {
	{
		switch value := value.value.(type) {
		case int8:
			return int32(value)
		case int16:
			return int32(value)
		case int32:
			return value
		}
	}
	floatValue := value.float64()
	if math.IsNaN(floatValue) || math.IsInf(floatValue, 0) {
		return 0
	}
	if floatValue == 0 { // This will work for +0 & -0
		return 0
	}
	remainder := math.Mod(floatValue, float_2_32)
	if remainder > 0 {
		remainder = math.Floor(remainder)
	} else {
		remainder = math.Ceil(remainder) + float_2_32
	}
	if remainder > float_2_31 {
		return int32(remainder - float_2_32)
	}
	return int32(remainder)
}

func toUint32(value Value) uint32 {
	{
		switch value := value.value.(type) {
		case int8:
			return uint32(value)
		case int16:
			return uint32(value)
		case uint8:
			return uint32(value)
		case uint16:
			return uint32(value)
		case uint32:
			return value
		}
	}
	floatValue := value.float64()
	if math.IsNaN(floatValue) || math.IsInf(floatValue, 0) {
		return 0
	}
	if floatValue == 0 {
		return 0
	}
	remainder := math.Mod(floatValue, float_2_32)
	if remainder > 0 {
		remainder = math.Floor(remainder)
	} else {
		remainder = math.Ceil(remainder) + float_2_32
	}
	return uint32(remainder)
}

func toUint16(value Value) uint16 {
	{
		switch value := value.value.(type) {
		case int8:
			return uint16(value)
		case uint8:
			return uint16(value)
		case uint16:
			return value
		}
	}
	floatValue := value.float64()
	if math.IsNaN(floatValue) || math.IsInf(floatValue, 0) {
		return 0
	}
	if floatValue == 0 {
		return 0
	}
	remainder := math.Mod(floatValue, float_2_16)
	if remainder > 0 {
		remainder = math.Floor(remainder)
	} else {
		remainder = math.Ceil(remainder) + float_2_16
	}
	return uint16(remainder)
}
