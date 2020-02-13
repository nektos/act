package parser

import (
	"bytes"
	"fmt"
	"strconv"
)

type _RegExp_parser struct {
	str    string
	length int

	chr       rune // The current character
	chrOffset int  // The offset of current character
	offset    int  // The offset after current character (may be greater than 1)

	errors  []error
	invalid bool // The input is an invalid JavaScript RegExp

	goRegexp *bytes.Buffer
}

// TransformRegExp transforms a JavaScript pattern into  a Go "regexp" pattern.
//
// re2 (Go) cannot do backtracking, so the presence of a lookahead (?=) (?!) or
// backreference (\1, \2, ...) will cause an error.
//
// re2 (Go) has a different definition for \s: [\t\n\f\r ].
// The JavaScript definition, on the other hand, also includes \v, Unicode "Separator, Space", etc.
//
// If the pattern is invalid (not valid even in JavaScript), then this function
// returns the empty string and an error.
//
// If the pattern is valid, but incompatible (contains a lookahead or backreference),
// then this function returns the transformation (a non-empty string) AND an error.
func TransformRegExp(pattern string) (string, error) {

	if pattern == "" {
		return "", nil
	}

	// TODO If without \, if without (?=, (?!, then another shortcut

	parser := _RegExp_parser{
		str:      pattern,
		length:   len(pattern),
		goRegexp: bytes.NewBuffer(make([]byte, 0, 3*len(pattern)/2)),
	}
	parser.read() // Pull in the first character
	parser.scan()
	var err error
	if len(parser.errors) > 0 {
		err = parser.errors[0]
	}
	if parser.invalid {
		return "", err
	}

	// Might not be re2 compatible, but is still a valid JavaScript RegExp
	return parser.goRegexp.String(), err
}

func (self *_RegExp_parser) scan() {
	for self.chr != -1 {
		switch self.chr {
		case '\\':
			self.read()
			self.scanEscape(false)
		case '(':
			self.pass()
			self.scanGroup()
		case '[':
			self.pass()
			self.scanBracket()
		case ')':
			self.error(-1, "Unmatched ')'")
			self.invalid = true
			self.pass()
		default:
			self.pass()
		}
	}
}

// (...)
func (self *_RegExp_parser) scanGroup() {
	str := self.str[self.chrOffset:]
	if len(str) > 1 { // A possibility of (?= or (?!
		if str[0] == '?' {
			if str[1] == '=' || str[1] == '!' {
				self.error(-1, "re2: Invalid (%s) <lookahead>", self.str[self.chrOffset:self.chrOffset+2])
			}
		}
	}
	for self.chr != -1 && self.chr != ')' {
		switch self.chr {
		case '\\':
			self.read()
			self.scanEscape(false)
		case '(':
			self.pass()
			self.scanGroup()
		case '[':
			self.pass()
			self.scanBracket()
		default:
			self.pass()
			continue
		}
	}
	if self.chr != ')' {
		self.error(-1, "Unterminated group")
		self.invalid = true
		return
	}
	self.pass()
}

// [...]
func (self *_RegExp_parser) scanBracket() {
	for self.chr != -1 {
		if self.chr == ']' {
			break
		} else if self.chr == '\\' {
			self.read()
			self.scanEscape(true)
			continue
		}
		self.pass()
	}
	if self.chr != ']' {
		self.error(-1, "Unterminated character class")
		self.invalid = true
		return
	}
	self.pass()
}

// \...
func (self *_RegExp_parser) scanEscape(inClass bool) {
	offset := self.chrOffset

	var length, base uint32
	switch self.chr {

	case '0', '1', '2', '3', '4', '5', '6', '7':
		var value int64
		size := 0
		for {
			digit := int64(digitValue(self.chr))
			if digit >= 8 {
				// Not a valid digit
				break
			}
			value = value*8 + digit
			self.read()
			size += 1
		}
		if size == 1 { // The number of characters read
			_, err := self.goRegexp.Write([]byte{'\\', byte(value) + '0'})
			if err != nil {
				self.errors = append(self.errors, err)
			}
			if value != 0 {
				// An invalid backreference
				self.error(-1, "re2: Invalid \\%d <backreference>", value)
			}
			return
		}
		tmp := []byte{'\\', 'x', '0', 0}
		if value >= 16 {
			tmp = tmp[0:2]
		} else {
			tmp = tmp[0:3]
		}
		tmp = strconv.AppendInt(tmp, value, 16)
		_, err := self.goRegexp.Write(tmp)
		if err != nil {
			self.errors = append(self.errors, err)
		}
		return

	case '8', '9':
		size := 0
		for {
			digit := digitValue(self.chr)
			if digit >= 10 {
				// Not a valid digit
				break
			}
			self.read()
			size += 1
		}
		err := self.goRegexp.WriteByte('\\')
		if err != nil {
			self.errors = append(self.errors, err)
		}
		_, err = self.goRegexp.WriteString(self.str[offset:self.chrOffset])
		if err != nil {
			self.errors = append(self.errors, err)
		}
		self.error(-1, "re2: Invalid \\%s <backreference>", self.str[offset:self.chrOffset])
		return

	case 'x':
		self.read()
		length, base = 2, 16

	case 'u':
		self.read()
		length, base = 4, 16

	case 'b':
		if inClass {
			_, err := self.goRegexp.Write([]byte{'\\', 'x', '0', '8'})
			if err != nil {
				self.errors = append(self.errors, err)
			}
			self.read()
			return
		}
		fallthrough

	case 'B':
		fallthrough

	case 'd', 'D', 's', 'S', 'w', 'W':
		// This is slightly broken, because ECMAScript
		// includes \v in \s, \S, while re2 does not
		fallthrough

	case '\\':
		fallthrough

	case 'f', 'n', 'r', 't', 'v':
		err := self.goRegexp.WriteByte('\\')
		if err != nil {
			self.errors = append(self.errors, err)
		}
		self.pass()
		return

	case 'c':
		self.read()
		var value int64
		if 'a' <= self.chr && self.chr <= 'z' {
			value = int64(self.chr) - 'a' + 1
		} else if 'A' <= self.chr && self.chr <= 'Z' {
			value = int64(self.chr) - 'A' + 1
		} else {
			err := self.goRegexp.WriteByte('c')
			if err != nil {
				self.errors = append(self.errors, err)
			}
			return
		}
		tmp := []byte{'\\', 'x', '0', 0}
		if value >= 16 {
			tmp = tmp[0:2]
		} else {
			tmp = tmp[0:3]
		}
		tmp = strconv.AppendInt(tmp, value, 16)
		_, err := self.goRegexp.Write(tmp)
		if err != nil {
			self.errors = append(self.errors, err)
		}
		self.read()
		return

	default:
		// $ is an identifier character, so we have to have
		// a special case for it here
		if self.chr == '$' || !isIdentifierPart(self.chr) {
			// A non-identifier character needs escaping
			err := self.goRegexp.WriteByte('\\')
			if err != nil {
				self.errors = append(self.errors, err)
			}
		} else {
			// Unescape the character for re2
		}
		self.pass()
		return
	}

	// Otherwise, we're a \u.... or \x...
	valueOffset := self.chrOffset

	var value uint32
	{
		length := length
		for ; length > 0; length-- {
			digit := uint32(digitValue(self.chr))
			if digit >= base {
				// Not a valid digit
				goto skip
			}
			value = value*base + digit
			self.read()
		}
	}

	if length == 4 {
		_, err := self.goRegexp.Write([]byte{
			'\\',
			'x',
			'{',
			self.str[valueOffset+0],
			self.str[valueOffset+1],
			self.str[valueOffset+2],
			self.str[valueOffset+3],
			'}',
		})
		if err != nil {
			self.errors = append(self.errors, err)
		}
	} else if length == 2 {
		_, err := self.goRegexp.Write([]byte{
			'\\',
			'x',
			self.str[valueOffset+0],
			self.str[valueOffset+1],
		})
		if err != nil {
			self.errors = append(self.errors, err)
		}
	} else {
		// Should never, ever get here...
		self.error(-1, "re2: Illegal branch in scanEscape")
		goto skip
	}

	return

skip:
	_, err := self.goRegexp.WriteString(self.str[offset:self.chrOffset])
	if err != nil {
		self.errors = append(self.errors, err)
	}
}

func (self *_RegExp_parser) pass() {
	if self.chr != -1 {
		_, err := self.goRegexp.WriteRune(self.chr)
		if err != nil {
			self.errors = append(self.errors, err)
		}
	}
	self.read()
}

// TODO Better error reporting, use the offset, etc.
func (self *_RegExp_parser) error(offset int, msg string, msgValues ...interface{}) error {
	err := fmt.Errorf(msg, msgValues...)
	self.errors = append(self.errors, err)
	return err
}
