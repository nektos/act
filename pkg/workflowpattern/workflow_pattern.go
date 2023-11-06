package workflowpattern

import (
	"fmt"
	"regexp"
	"strings"
)

type WorkflowPattern struct {
	Pattern  string
	Negative bool
	Regex    *regexp.Regexp
}

func CompilePattern(rawpattern string) (*WorkflowPattern, error) {
	negative := false
	pattern := rawpattern
	if strings.HasPrefix(rawpattern, "!") {
		negative = true
		pattern = rawpattern[1:]
	}
	rpattern, err := PatternToRegex(pattern)
	if err != nil {
		return nil, err
	}
	regex, err := regexp.Compile(rpattern)
	if err != nil {
		return nil, err
	}
	return &WorkflowPattern{
		Pattern:  pattern,
		Negative: negative,
		Regex:    regex,
	}, nil
}

//nolint:gocyclo
func PatternToRegex(pattern string) (string, error) {
	var rpattern strings.Builder
	rpattern.WriteString("^")
	pos := 0
	errors := map[int]string{}
	for pos < len(pattern) {
		switch pattern[pos] {
		case '*':
			if pos+1 < len(pattern) && pattern[pos+1] == '*' {
				if pos+2 < len(pattern) && pattern[pos+2] == '/' {
					rpattern.WriteString("(.+/)?")
					pos += 3
				} else {
					rpattern.WriteString(".*")
					pos += 2
				}
			} else {
				rpattern.WriteString("[^/]*")
				pos++
			}
		case '+', '?':
			if pos > 0 {
				rpattern.WriteByte(pattern[pos])
			} else {
				rpattern.WriteString(regexp.QuoteMeta(string([]byte{pattern[pos]})))
			}
			pos++
		case '[':
			rpattern.WriteByte(pattern[pos])
			pos++
			if pos < len(pattern) && pattern[pos] == ']' {
				errors[pos] = "Unexpected empty brackets '[]'"
				pos++
				break
			}
			validChar := func(a, b, test byte) bool {
				return test >= a && test <= b
			}
			startPos := pos
			for pos < len(pattern) && pattern[pos] != ']' {
				switch pattern[pos] {
				case '-':
					if pos <= startPos || pos+1 >= len(pattern) {
						errors[pos] = "Invalid range"
						pos++
						break
					}
					validRange := func(a, b byte) bool {
						return validChar(a, b, pattern[pos-1]) && validChar(a, b, pattern[pos+1]) && pattern[pos-1] <= pattern[pos+1]
					}
					if !validRange('A', 'z') && !validRange('0', '9') {
						errors[pos] = "Ranges can only include a-z, A-Z, A-z, and 0-9"
						pos++
						break
					}
					rpattern.WriteString(pattern[pos : pos+2])
					pos += 2
				default:
					if !validChar('A', 'z', pattern[pos]) && !validChar('0', '9', pattern[pos]) {
						errors[pos] = "Ranges can only include a-z, A-Z and 0-9"
						pos++
						break
					}
					rpattern.WriteString(regexp.QuoteMeta(string([]byte{pattern[pos]})))
					pos++
				}
			}
			if pos >= len(pattern) || pattern[pos] != ']' {
				errors[pos] = "Missing closing bracket ']' after '['"
				pos++
			}
			rpattern.WriteString("]")
			pos++
		case '\\':
			if pos+1 >= len(pattern) {
				errors[pos] = "Missing symbol after \\"
				pos++
				break
			}
			rpattern.WriteString(regexp.QuoteMeta(string([]byte{pattern[pos+1]})))
			pos += 2
		default:
			rpattern.WriteString(regexp.QuoteMeta(string([]byte{pattern[pos]})))
			pos++
		}
	}
	if len(errors) > 0 {
		var errorMessage strings.Builder
		for position, err := range errors {
			if errorMessage.Len() > 0 {
				errorMessage.WriteString(", ")
			}
			errorMessage.WriteString(fmt.Sprintf("Position: %d Error: %s", position, err))
		}
		return "", fmt.Errorf("invalid Pattern '%s': %s", pattern, errorMessage.String())
	}
	rpattern.WriteString("$")
	return rpattern.String(), nil
}

func CompilePatterns(patterns ...string) ([]*WorkflowPattern, error) {
	ret := []*WorkflowPattern{}
	for _, pattern := range patterns {
		cp, err := CompilePattern(pattern)
		if err != nil {
			return nil, err
		}
		ret = append(ret, cp)
	}
	return ret, nil
}

//FilterInputsFunc defines the signature that both Skip() and Filter() implement
type FilterInputsFunc func(sequence []*WorkflowPattern, input []string, traceWriter TraceWriter) bool

// returns true if the workflow should be skipped paths/branches
func Skip(sequence []*WorkflowPattern, input []string, traceWriter TraceWriter) bool {
	if len(sequence) == 0 {
		return false
	}
	for _, file := range input {
		matched := false
		for _, item := range sequence {
			if item.Regex.MatchString(file) {
				pattern := item.Pattern
				if item.Negative {
					matched = false
					traceWriter.Info("%s excluded by pattern %s", file, pattern)
				} else {
					matched = true
					traceWriter.Info("%s included by pattern %s", file, pattern)
				}
			}
		}
		if matched {
			return false
		}
	}
	return true
}

// returns true if the workflow should be skipped paths-ignore/branches-ignore
func Filter(sequence []*WorkflowPattern, input []string, traceWriter TraceWriter) bool {
	if len(sequence) == 0 {
		return false
	}
	for _, file := range input {
		matched := false
		for _, item := range sequence {
			if item.Regex.MatchString(file) == !item.Negative {
				pattern := item.Pattern
				traceWriter.Info("%s ignored by pattern %s", file, pattern)
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
