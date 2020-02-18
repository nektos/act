package otto

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// String

func stringValueFromStringArgumentList(argumentList []Value) Value {
	if len(argumentList) > 0 {
		return toValue_string(argumentList[0].string())
	}
	return toValue_string("")
}

func builtinString(call FunctionCall) Value {
	return stringValueFromStringArgumentList(call.ArgumentList)
}

func builtinNewString(self *_object, argumentList []Value) Value {
	return toValue_object(self.runtime.newString(stringValueFromStringArgumentList(argumentList)))
}

func builtinString_toString(call FunctionCall) Value {
	return call.thisClassObject("String").primitiveValue()
}
func builtinString_valueOf(call FunctionCall) Value {
	return call.thisClassObject("String").primitiveValue()
}

func builtinString_fromCharCode(call FunctionCall) Value {
	chrList := make([]uint16, len(call.ArgumentList))
	for index, value := range call.ArgumentList {
		chrList[index] = toUint16(value)
	}
	return toValue_string16(chrList)
}

func builtinString_charAt(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	idx := int(call.Argument(0).number().int64)
	chr := stringAt(call.This._object().stringValue(), idx)
	if chr == utf8.RuneError {
		return toValue_string("")
	}
	return toValue_string(string(chr))
}

func builtinString_charCodeAt(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	idx := int(call.Argument(0).number().int64)
	chr := stringAt(call.This._object().stringValue(), idx)
	if chr == utf8.RuneError {
		return NaNValue()
	}
	return toValue_uint16(uint16(chr))
}

func builtinString_concat(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	var value bytes.Buffer
	value.WriteString(call.This.string())
	for _, item := range call.ArgumentList {
		value.WriteString(item.string())
	}
	return toValue_string(value.String())
}

func builtinString_indexOf(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	value := call.This.string()
	target := call.Argument(0).string()
	if 2 > len(call.ArgumentList) {
		return toValue_int(strings.Index(value, target))
	}
	start := toIntegerFloat(call.Argument(1))
	if 0 > start {
		start = 0
	} else if start >= float64(len(value)) {
		if target == "" {
			return toValue_int(len(value))
		}
		return toValue_int(-1)
	}
	index := strings.Index(value[int(start):], target)
	if index >= 0 {
		index += int(start)
	}
	return toValue_int(index)
}

func builtinString_lastIndexOf(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	value := call.This.string()
	target := call.Argument(0).string()
	if 2 > len(call.ArgumentList) || call.ArgumentList[1].IsUndefined() {
		return toValue_int(strings.LastIndex(value, target))
	}
	length := len(value)
	if length == 0 {
		return toValue_int(strings.LastIndex(value, target))
	}
	start := call.ArgumentList[1].number()
	if start.kind == numberInfinity { // FIXME
		// startNumber is infinity, so start is the end of string (start = length)
		return toValue_int(strings.LastIndex(value, target))
	}
	if 0 > start.int64 {
		start.int64 = 0
	}
	end := int(start.int64) + len(target)
	if end > length {
		end = length
	}
	return toValue_int(strings.LastIndex(value[:end], target))
}

func builtinString_match(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	target := call.This.string()
	matcherValue := call.Argument(0)
	matcher := matcherValue._object()
	if !matcherValue.IsObject() || matcher.class != "RegExp" {
		matcher = call.runtime.newRegExp(matcherValue, Value{})
	}
	global := matcher.get("global").bool()
	if !global {
		match, result := execRegExp(matcher, target)
		if !match {
			return nullValue
		}
		return toValue_object(execResultToArray(call.runtime, target, result))
	}

	{
		result := matcher.regExpValue().regularExpression.FindAllStringIndex(target, -1)
		matchCount := len(result)
		if result == nil {
			matcher.put("lastIndex", toValue_int(0), true)
			return Value{} // !match
		}
		matchCount = len(result)
		valueArray := make([]Value, matchCount)
		for index := 0; index < matchCount; index++ {
			valueArray[index] = toValue_string(target[result[index][0]:result[index][1]])
		}
		matcher.put("lastIndex", toValue_int(result[matchCount-1][1]), true)
		return toValue_object(call.runtime.newArrayOf(valueArray))
	}
}

var builtinString_replace_Regexp = regexp.MustCompile("\\$(?:[\\$\\&\\'\\`1-9]|0[1-9]|[1-9][0-9])")

func builtinString_findAndReplaceString(input []byte, lastIndex int, match []int, target []byte, replaceValue []byte) (output []byte) {
	matchCount := len(match) / 2
	output = input
	if match[0] != lastIndex {
		output = append(output, target[lastIndex:match[0]]...)
	}
	replacement := builtinString_replace_Regexp.ReplaceAllFunc(replaceValue, func(part []byte) []byte {
		// TODO Check if match[0] or match[1] can be -1 in this scenario
		switch part[1] {
		case '$':
			return []byte{'$'}
		case '&':
			return target[match[0]:match[1]]
		case '`':
			return target[:match[0]]
		case '\'':
			return target[match[1]:len(target)]
		}
		matchNumberParse, error := strconv.ParseInt(string(part[1:]), 10, 64)
		matchNumber := int(matchNumberParse)
		if error != nil || matchNumber >= matchCount {
			return []byte{}
		}
		offset := 2 * matchNumber
		if match[offset] != -1 {
			return target[match[offset]:match[offset+1]]
		}
		return []byte{} // The empty string
	})
	output = append(output, replacement...)
	return output
}

func builtinString_replace(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	target := []byte(call.This.string())
	searchValue := call.Argument(0)
	searchObject := searchValue._object()

	// TODO If a capture is -1?
	var search *regexp.Regexp
	global := false
	find := 1
	if searchValue.IsObject() && searchObject.class == "RegExp" {
		regExp := searchObject.regExpValue()
		search = regExp.regularExpression
		if regExp.global {
			find = -1
		}
	} else {
		search = regexp.MustCompile(regexp.QuoteMeta(searchValue.string()))
	}

	found := search.FindAllSubmatchIndex(target, find)
	if found == nil {
		return toValue_string(string(target)) // !match
	}

	{
		lastIndex := 0
		result := []byte{}

		replaceValue := call.Argument(1)
		if replaceValue.isCallable() {
			target := string(target)
			replace := replaceValue._object()
			for _, match := range found {
				if match[0] != lastIndex {
					result = append(result, target[lastIndex:match[0]]...)
				}
				matchCount := len(match) / 2
				argumentList := make([]Value, matchCount+2)
				for index := 0; index < matchCount; index++ {
					offset := 2 * index
					if match[offset] != -1 {
						argumentList[index] = toValue_string(target[match[offset]:match[offset+1]])
					} else {
						argumentList[index] = Value{}
					}
				}
				argumentList[matchCount+0] = toValue_int(match[0])
				argumentList[matchCount+1] = toValue_string(target)
				replacement := replace.call(Value{}, argumentList, false, nativeFrame).string()
				result = append(result, []byte(replacement)...)
				lastIndex = match[1]
			}

		} else {
			replace := []byte(replaceValue.string())
			for _, match := range found {
				result = builtinString_findAndReplaceString(result, lastIndex, match, target, replace)
				lastIndex = match[1]
			}
		}

		if lastIndex != len(target) {
			result = append(result, target[lastIndex:]...)
		}

		if global && searchObject != nil {
			searchObject.put("lastIndex", toValue_int(lastIndex), true)
		}

		return toValue_string(string(result))
	}
}

func builtinString_search(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	target := call.This.string()
	searchValue := call.Argument(0)
	search := searchValue._object()
	if !searchValue.IsObject() || search.class != "RegExp" {
		search = call.runtime.newRegExp(searchValue, Value{})
	}
	result := search.regExpValue().regularExpression.FindStringIndex(target)
	if result == nil {
		return toValue_int(-1)
	}
	return toValue_int(result[0])
}

func stringSplitMatch(target string, targetLength int64, index uint, search string, searchLength int64) (bool, uint) {
	if int64(index)+searchLength > searchLength {
		return false, 0
	}
	found := strings.Index(target[index:], search)
	if 0 > found {
		return false, 0
	}
	return true, uint(found)
}

func builtinString_split(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	target := call.This.string()

	separatorValue := call.Argument(0)
	limitValue := call.Argument(1)
	limit := -1
	if limitValue.IsDefined() {
		limit = int(toUint32(limitValue))
	}

	if limit == 0 {
		return toValue_object(call.runtime.newArray(0))
	}

	if separatorValue.IsUndefined() {
		return toValue_object(call.runtime.newArrayOf([]Value{toValue_string(target)}))
	}

	if separatorValue.isRegExp() {
		targetLength := len(target)
		search := separatorValue._object().regExpValue().regularExpression
		valueArray := []Value{}
		result := search.FindAllStringSubmatchIndex(target, -1)
		lastIndex := 0
		found := 0

		for _, match := range result {
			if match[0] == match[1] {
				// FIXME Ugh, this is a hack
				if match[0] == 0 || match[0] == targetLength {
					continue
				}
			}

			if lastIndex != match[0] {
				valueArray = append(valueArray, toValue_string(target[lastIndex:match[0]]))
				found++
			} else if lastIndex == match[0] {
				if lastIndex != -1 {
					valueArray = append(valueArray, toValue_string(""))
					found++
				}
			}

			lastIndex = match[1]
			if found == limit {
				goto RETURN
			}

			captureCount := len(match) / 2
			for index := 1; index < captureCount; index++ {
				offset := index * 2
				value := Value{}
				if match[offset] != -1 {
					value = toValue_string(target[match[offset]:match[offset+1]])
				}
				valueArray = append(valueArray, value)
				found++
				if found == limit {
					goto RETURN
				}
			}
		}

		if found != limit {
			if lastIndex != targetLength {
				valueArray = append(valueArray, toValue_string(target[lastIndex:targetLength]))
			} else {
				valueArray = append(valueArray, toValue_string(""))
			}
		}

	RETURN:
		return toValue_object(call.runtime.newArrayOf(valueArray))

	} else {
		separator := separatorValue.string()

		splitLimit := limit
		excess := false
		if limit > 0 {
			splitLimit = limit + 1
			excess = true
		}

		split := strings.SplitN(target, separator, splitLimit)

		if excess && len(split) > limit {
			split = split[:limit]
		}

		valueArray := make([]Value, len(split))
		for index, value := range split {
			valueArray[index] = toValue_string(value)
		}

		return toValue_object(call.runtime.newArrayOf(valueArray))
	}
}

func builtinString_slice(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	target := call.This.string()

	length := int64(len(target))
	start, end := rangeStartEnd(call.ArgumentList, length, false)
	if end-start <= 0 {
		return toValue_string("")
	}
	return toValue_string(target[start:end])
}

func builtinString_substring(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	target := call.This.string()

	length := int64(len(target))
	start, end := rangeStartEnd(call.ArgumentList, length, true)
	if start > end {
		start, end = end, start
	}
	return toValue_string(target[start:end])
}

func builtinString_substr(call FunctionCall) Value {
	target := call.This.string()

	size := int64(len(target))
	start, length := rangeStartLength(call.ArgumentList, size)

	if start >= size {
		return toValue_string("")
	}

	if length <= 0 {
		return toValue_string("")
	}

	if start+length >= size {
		// Cap length to be to the end of the string
		// start = 3, length = 5, size = 4 [0, 1, 2, 3]
		// 4 - 3 = 1
		// target[3:4]
		length = size - start
	}

	return toValue_string(target[start : start+length])
}

func builtinString_toLowerCase(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	return toValue_string(strings.ToLower(call.This.string()))
}

func builtinString_toUpperCase(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	return toValue_string(strings.ToUpper(call.This.string()))
}

// 7.2 Table 2 — Whitespace Characters & 7.3 Table 3 - Line Terminator Characters
const builtinString_trim_whitespace = "\u0009\u000A\u000B\u000C\u000D\u0020\u00A0\u1680\u180E\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200A\u2028\u2029\u202F\u205F\u3000\uFEFF"

func builtinString_trim(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	return toValue(strings.Trim(call.This.string(),
		builtinString_trim_whitespace))
}

// Mozilla extension, not ECMAScript 5
func builtinString_trimLeft(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	return toValue(strings.TrimLeft(call.This.string(),
		builtinString_trim_whitespace))
}

// Mozilla extension, not ECMAScript 5
func builtinString_trimRight(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	return toValue(strings.TrimRight(call.This.string(),
		builtinString_trim_whitespace))
}

func builtinString_localeCompare(call FunctionCall) Value {
	checkObjectCoercible(call.runtime, call.This)
	this := call.This.string()
	that := call.Argument(0).string()
	if this < that {
		return toValue_int(-1)
	} else if this == that {
		return toValue_int(0)
	}
	return toValue_int(1)
}

/*
An alternate version of String.trim
func builtinString_trim(call FunctionCall) Value {
	checkObjectCoercible(call.This)
	return toValue_string(strings.TrimFunc(call.string(.This), isWhiteSpaceOrLineTerminator))
}
*/

func builtinString_toLocaleLowerCase(call FunctionCall) Value {
	return builtinString_toLowerCase(call)
}

func builtinString_toLocaleUpperCase(call FunctionCall) Value {
	return builtinString_toUpperCase(call)
}
