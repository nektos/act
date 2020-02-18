package otto

import (
	"encoding/hex"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Global
func builtinGlobal_eval(call FunctionCall) Value {
	src := call.Argument(0)
	if !src.IsString() {
		return src
	}
	runtime := call.runtime
	program := runtime.cmpl_parseOrThrow(src.string(), nil)
	if !call.eval {
		// Not a direct call to eval, so we enter the global ExecutionContext
		runtime.enterGlobalScope()
		defer runtime.leaveScope()
	}
	returnValue := runtime.cmpl_evaluate_nodeProgram(program, true)
	if returnValue.isEmpty() {
		return Value{}
	}
	return returnValue
}

func builtinGlobal_isNaN(call FunctionCall) Value {
	value := call.Argument(0).float64()
	return toValue_bool(math.IsNaN(value))
}

func builtinGlobal_isFinite(call FunctionCall) Value {
	value := call.Argument(0).float64()
	return toValue_bool(!math.IsNaN(value) && !math.IsInf(value, 0))
}

// radix 3 => 2 (ASCII 50) +47
// radix 11 => A/a (ASCII 65/97) +54/+86
var parseInt_alphabetTable = func() []string {
	table := []string{"", "", "01"}
	for radix := 3; radix <= 36; radix += 1 {
		alphabet := table[radix-1]
		if radix <= 10 {
			alphabet += string(radix + 47)
		} else {
			alphabet += string(radix+54) + string(radix+86)
		}
		table = append(table, alphabet)
	}
	return table
}()

func digitValue(chr rune) int {
	switch {
	case '0' <= chr && chr <= '9':
		return int(chr - '0')
	case 'a' <= chr && chr <= 'z':
		return int(chr - 'a' + 10)
	case 'A' <= chr && chr <= 'Z':
		return int(chr - 'A' + 10)
	}
	return 36 // Larger than any legal digit value
}

func builtinGlobal_parseInt(call FunctionCall) Value {
	input := strings.Trim(call.Argument(0).string(), builtinString_trim_whitespace)
	if len(input) == 0 {
		return NaNValue()
	}

	radix := int(toInt32(call.Argument(1)))

	negative := false
	switch input[0] {
	case '+':
		input = input[1:]
	case '-':
		negative = true
		input = input[1:]
	}

	strip := true
	if radix == 0 {
		radix = 10
	} else {
		if radix < 2 || radix > 36 {
			return NaNValue()
		} else if radix != 16 {
			strip = false
		}
	}

	switch len(input) {
	case 0:
		return NaNValue()
	case 1:
	default:
		if strip {
			if input[0] == '0' && (input[1] == 'x' || input[1] == 'X') {
				input = input[2:]
				radix = 16
			}
		}
	}

	base := radix
	index := 0
	for ; index < len(input); index++ {
		digit := digitValue(rune(input[index])) // If not ASCII, then an error anyway
		if digit >= base {
			break
		}
	}
	input = input[0:index]

	value, err := strconv.ParseInt(input, radix, 64)
	if err != nil {
		if err.(*strconv.NumError).Err == strconv.ErrRange {
			base := float64(base)
			// Could just be a very large number (e.g. 0x8000000000000000)
			var value float64
			for _, chr := range input {
				digit := float64(digitValue(chr))
				if digit >= base {
					goto error
				}
				value = value*base + digit
			}
			if negative {
				value *= -1
			}
			return toValue_float64(value)
		}
	error:
		return NaNValue()
	}
	if negative {
		value *= -1
	}

	return toValue_int64(value)
}

var parseFloat_matchBadSpecial = regexp.MustCompile(`[\+\-]?(?:[Ii]nf$|infinity)`)
var parseFloat_matchValid = regexp.MustCompile(`[0-9eE\+\-\.]|Infinity`)

func builtinGlobal_parseFloat(call FunctionCall) Value {
	// Caveat emptor: This implementation does NOT match the specification
	input := strings.Trim(call.Argument(0).string(), builtinString_trim_whitespace)

	if parseFloat_matchBadSpecial.MatchString(input) {
		return NaNValue()
	}
	value, err := strconv.ParseFloat(input, 64)
	if err != nil {
		for end := len(input); end > 0; end-- {
			input := input[0:end]
			if !parseFloat_matchValid.MatchString(input) {
				return NaNValue()
			}
			value, err = strconv.ParseFloat(input, 64)
			if err == nil {
				break
			}
		}
		if err != nil {
			return NaNValue()
		}
	}
	return toValue_float64(value)
}

// encodeURI/decodeURI

func _builtinGlobal_encodeURI(call FunctionCall, escape *regexp.Regexp) Value {
	value := call.Argument(0)
	var input []uint16
	switch vl := value.value.(type) {
	case []uint16:
		input = vl
	default:
		input = utf16.Encode([]rune(value.string()))
	}
	if len(input) == 0 {
		return toValue_string("")
	}
	output := []byte{}
	length := len(input)
	encode := make([]byte, 4)
	for index := 0; index < length; {
		value := input[index]
		decode := utf16.Decode(input[index : index+1])
		if value >= 0xDC00 && value <= 0xDFFF {
			panic(call.runtime.panicURIError("URI malformed"))
		}
		if value >= 0xD800 && value <= 0xDBFF {
			index += 1
			if index >= length {
				panic(call.runtime.panicURIError("URI malformed"))
			}
			// input = ..., value, value1, ...
			value1 := input[index]
			if value1 < 0xDC00 || value1 > 0xDFFF {
				panic(call.runtime.panicURIError("URI malformed"))
			}
			decode = []rune{((rune(value) - 0xD800) * 0x400) + (rune(value1) - 0xDC00) + 0x10000}
		}
		index += 1
		size := utf8.EncodeRune(encode, decode[0])
		encode := encode[0:size]
		output = append(output, encode...)
	}
	{
		value := escape.ReplaceAllFunc(output, func(target []byte) []byte {
			// Probably a better way of doing this
			if target[0] == ' ' {
				return []byte("%20")
			}
			return []byte(url.QueryEscape(string(target)))
		})
		return toValue_string(string(value))
	}
}

var encodeURI_Regexp = regexp.MustCompile(`([^~!@#$&*()=:/,;?+'])`)

func builtinGlobal_encodeURI(call FunctionCall) Value {
	return _builtinGlobal_encodeURI(call, encodeURI_Regexp)
}

var encodeURIComponent_Regexp = regexp.MustCompile(`([^~!*()'])`)

func builtinGlobal_encodeURIComponent(call FunctionCall) Value {
	return _builtinGlobal_encodeURI(call, encodeURIComponent_Regexp)
}

// 3B/2F/3F/3A/40/26/3D/2B/24/2C/23
var decodeURI_guard = regexp.MustCompile(`(?i)(?:%)(3B|2F|3F|3A|40|26|3D|2B|24|2C|23)`)

func _decodeURI(input string, reserve bool) (string, bool) {
	if reserve {
		input = decodeURI_guard.ReplaceAllString(input, "%25$1")
	}
	input = strings.Replace(input, "+", "%2B", -1) // Ugly hack to make QueryUnescape work with our use case
	output, err := url.QueryUnescape(input)
	if err != nil || !utf8.ValidString(output) {
		return "", true
	}
	return output, false
}

func builtinGlobal_decodeURI(call FunctionCall) Value {
	output, err := _decodeURI(call.Argument(0).string(), true)
	if err {
		panic(call.runtime.panicURIError("URI malformed"))
	}
	return toValue_string(output)
}

func builtinGlobal_decodeURIComponent(call FunctionCall) Value {
	output, err := _decodeURI(call.Argument(0).string(), false)
	if err {
		panic(call.runtime.panicURIError("URI malformed"))
	}
	return toValue_string(output)
}

// escape/unescape

func builtin_shouldEscape(chr byte) bool {
	if 'A' <= chr && chr <= 'Z' || 'a' <= chr && chr <= 'z' || '0' <= chr && chr <= '9' {
		return false
	}
	return !strings.ContainsRune("*_+-./", rune(chr))
}

const escapeBase16 = "0123456789ABCDEF"

func builtin_escape(input string) string {
	output := make([]byte, 0, len(input))
	length := len(input)
	for index := 0; index < length; {
		if builtin_shouldEscape(input[index]) {
			chr, width := utf8.DecodeRuneInString(input[index:])
			chr16 := utf16.Encode([]rune{chr})[0]
			if 256 > chr16 {
				output = append(output, '%',
					escapeBase16[chr16>>4],
					escapeBase16[chr16&15],
				)
			} else {
				output = append(output, '%', 'u',
					escapeBase16[chr16>>12],
					escapeBase16[(chr16>>8)&15],
					escapeBase16[(chr16>>4)&15],
					escapeBase16[chr16&15],
				)
			}
			index += width

		} else {
			output = append(output, input[index])
			index += 1
		}
	}
	return string(output)
}

func builtin_unescape(input string) string {
	output := make([]rune, 0, len(input))
	length := len(input)
	for index := 0; index < length; {
		if input[index] == '%' {
			if index <= length-6 && input[index+1] == 'u' {
				byte16, err := hex.DecodeString(input[index+2 : index+6])
				if err == nil {
					value := uint16(byte16[0])<<8 + uint16(byte16[1])
					chr := utf16.Decode([]uint16{value})[0]
					output = append(output, chr)
					index += 6
					continue
				}
			}
			if index <= length-3 {
				byte8, err := hex.DecodeString(input[index+1 : index+3])
				if err == nil {
					value := uint16(byte8[0])
					chr := utf16.Decode([]uint16{value})[0]
					output = append(output, chr)
					index += 3
					continue
				}
			}
		}
		output = append(output, rune(input[index]))
		index += 1
	}
	return string(output)
}

func builtinGlobal_escape(call FunctionCall) Value {
	return toValue_string(builtin_escape(call.Argument(0).string()))
}

func builtinGlobal_unescape(call FunctionCall) Value {
	return toValue_string(builtin_unescape(call.Argument(0).string()))
}
