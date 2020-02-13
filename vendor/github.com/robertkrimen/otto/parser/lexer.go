package parser

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/robertkrimen/otto/ast"
	"github.com/robertkrimen/otto/file"
	"github.com/robertkrimen/otto/token"
)

type _chr struct {
	value rune
	width int
}

var matchIdentifier = regexp.MustCompile(`^[$_\p{L}][$_\p{L}\d}]*$`)

func isDecimalDigit(chr rune) bool {
	return '0' <= chr && chr <= '9'
}

func digitValue(chr rune) int {
	switch {
	case '0' <= chr && chr <= '9':
		return int(chr - '0')
	case 'a' <= chr && chr <= 'f':
		return int(chr - 'a' + 10)
	case 'A' <= chr && chr <= 'F':
		return int(chr - 'A' + 10)
	}
	return 16 // Larger than any legal digit value
}

func isDigit(chr rune, base int) bool {
	return digitValue(chr) < base
}

func isIdentifierStart(chr rune) bool {
	return chr == '$' || chr == '_' || chr == '\\' ||
		'a' <= chr && chr <= 'z' || 'A' <= chr && chr <= 'Z' ||
		chr >= utf8.RuneSelf && unicode.IsLetter(chr)
}

func isIdentifierPart(chr rune) bool {
	return chr == '$' || chr == '_' || chr == '\\' ||
		'a' <= chr && chr <= 'z' || 'A' <= chr && chr <= 'Z' ||
		'0' <= chr && chr <= '9' ||
		chr >= utf8.RuneSelf && (unicode.IsLetter(chr) || unicode.IsDigit(chr))
}

func (self *_parser) scanIdentifier() (string, error) {
	offset := self.chrOffset
	parse := false
	for isIdentifierPart(self.chr) {
		if self.chr == '\\' {
			distance := self.chrOffset - offset
			self.read()
			if self.chr != 'u' {
				return "", fmt.Errorf("Invalid identifier escape character: %c (%s)", self.chr, string(self.chr))
			}
			parse = true
			var value rune
			for j := 0; j < 4; j++ {
				self.read()
				decimal, ok := hex2decimal(byte(self.chr))
				if !ok {
					return "", fmt.Errorf("Invalid identifier escape character: %c (%s)", self.chr, string(self.chr))
				}
				value = value<<4 | decimal
			}
			if value == '\\' {
				return "", fmt.Errorf("Invalid identifier escape value: %c (%s)", value, string(value))
			} else if distance == 0 {
				if !isIdentifierStart(value) {
					return "", fmt.Errorf("Invalid identifier escape value: %c (%s)", value, string(value))
				}
			} else if distance > 0 {
				if !isIdentifierPart(value) {
					return "", fmt.Errorf("Invalid identifier escape value: %c (%s)", value, string(value))
				}
			}
		}
		self.read()
	}
	literal := string(self.str[offset:self.chrOffset])
	if parse {
		return parseStringLiteral(literal)
	}
	return literal, nil
}

// 7.2
func isLineWhiteSpace(chr rune) bool {
	switch chr {
	case '\u0009', '\u000b', '\u000c', '\u0020', '\u00a0', '\ufeff':
		return true
	case '\u000a', '\u000d', '\u2028', '\u2029':
		return false
	case '\u0085':
		return false
	}
	return unicode.IsSpace(chr)
}

// 7.3
func isLineTerminator(chr rune) bool {
	switch chr {
	case '\u000a', '\u000d', '\u2028', '\u2029':
		return true
	}
	return false
}

func (self *_parser) scan() (tkn token.Token, literal string, idx file.Idx) {

	self.implicitSemicolon = false

	for {
		self.skipWhiteSpace()

		idx = self.idxOf(self.chrOffset)
		insertSemicolon := false

		switch chr := self.chr; {
		case isIdentifierStart(chr):
			var err error
			literal, err = self.scanIdentifier()
			if err != nil {
				tkn = token.ILLEGAL
				break
			}
			if len(literal) > 1 {
				// Keywords are longer than 1 character, avoid lookup otherwise
				var strict bool
				tkn, strict = token.IsKeyword(literal)

				switch tkn {

				case 0: // Not a keyword
					if literal == "true" || literal == "false" {
						self.insertSemicolon = true
						tkn = token.BOOLEAN
						return
					} else if literal == "null" {
						self.insertSemicolon = true
						tkn = token.NULL
						return
					}

				case token.KEYWORD:
					tkn = token.KEYWORD
					if strict {
						// TODO If strict and in strict mode, then this is not a break
						break
					}
					return

				case
					token.THIS,
					token.BREAK,
					token.THROW, // A newline after a throw is not allowed, but we need to detect it
					token.RETURN,
					token.CONTINUE,
					token.DEBUGGER:
					self.insertSemicolon = true
					return

				default:
					return

				}
			}
			self.insertSemicolon = true
			tkn = token.IDENTIFIER
			return
		case '0' <= chr && chr <= '9':
			self.insertSemicolon = true
			tkn, literal = self.scanNumericLiteral(false)
			return
		default:
			self.read()
			switch chr {
			case -1:
				if self.insertSemicolon {
					self.insertSemicolon = false
					self.implicitSemicolon = true
				}
				tkn = token.EOF
			case '\r', '\n', '\u2028', '\u2029':
				self.insertSemicolon = false
				self.implicitSemicolon = true
				self.comments.AtLineBreak()
				continue
			case ':':
				tkn = token.COLON
			case '.':
				if digitValue(self.chr) < 10 {
					insertSemicolon = true
					tkn, literal = self.scanNumericLiteral(true)
				} else {
					tkn = token.PERIOD
				}
			case ',':
				tkn = token.COMMA
			case ';':
				tkn = token.SEMICOLON
			case '(':
				tkn = token.LEFT_PARENTHESIS
			case ')':
				tkn = token.RIGHT_PARENTHESIS
				insertSemicolon = true
			case '[':
				tkn = token.LEFT_BRACKET
			case ']':
				tkn = token.RIGHT_BRACKET
				insertSemicolon = true
			case '{':
				tkn = token.LEFT_BRACE
			case '}':
				tkn = token.RIGHT_BRACE
				insertSemicolon = true
			case '+':
				tkn = self.switch3(token.PLUS, token.ADD_ASSIGN, '+', token.INCREMENT)
				if tkn == token.INCREMENT {
					insertSemicolon = true
				}
			case '-':
				tkn = self.switch3(token.MINUS, token.SUBTRACT_ASSIGN, '-', token.DECREMENT)
				if tkn == token.DECREMENT {
					insertSemicolon = true
				}
			case '*':
				tkn = self.switch2(token.MULTIPLY, token.MULTIPLY_ASSIGN)
			case '/':
				if self.chr == '/' {
					if self.mode&StoreComments != 0 {
						literal := string(self.readSingleLineComment())
						self.comments.AddComment(ast.NewComment(literal, self.idx))
						continue
					}
					self.skipSingleLineComment()
					continue
				} else if self.chr == '*' {
					if self.mode&StoreComments != 0 {
						literal = string(self.readMultiLineComment())
						self.comments.AddComment(ast.NewComment(literal, self.idx))
						continue
					}
					self.skipMultiLineComment()
					continue
				} else {
					// Could be division, could be RegExp literal
					tkn = self.switch2(token.SLASH, token.QUOTIENT_ASSIGN)
					insertSemicolon = true
				}
			case '%':
				tkn = self.switch2(token.REMAINDER, token.REMAINDER_ASSIGN)
			case '^':
				tkn = self.switch2(token.EXCLUSIVE_OR, token.EXCLUSIVE_OR_ASSIGN)
			case '<':
				tkn = self.switch4(token.LESS, token.LESS_OR_EQUAL, '<', token.SHIFT_LEFT, token.SHIFT_LEFT_ASSIGN)
			case '>':
				tkn = self.switch6(token.GREATER, token.GREATER_OR_EQUAL, '>', token.SHIFT_RIGHT, token.SHIFT_RIGHT_ASSIGN, '>', token.UNSIGNED_SHIFT_RIGHT, token.UNSIGNED_SHIFT_RIGHT_ASSIGN)
			case '=':
				tkn = self.switch2(token.ASSIGN, token.EQUAL)
				if tkn == token.EQUAL && self.chr == '=' {
					self.read()
					tkn = token.STRICT_EQUAL
				}
			case '!':
				tkn = self.switch2(token.NOT, token.NOT_EQUAL)
				if tkn == token.NOT_EQUAL && self.chr == '=' {
					self.read()
					tkn = token.STRICT_NOT_EQUAL
				}
			case '&':
				if self.chr == '^' {
					self.read()
					tkn = self.switch2(token.AND_NOT, token.AND_NOT_ASSIGN)
				} else {
					tkn = self.switch3(token.AND, token.AND_ASSIGN, '&', token.LOGICAL_AND)
				}
			case '|':
				tkn = self.switch3(token.OR, token.OR_ASSIGN, '|', token.LOGICAL_OR)
			case '~':
				tkn = token.BITWISE_NOT
			case '?':
				tkn = token.QUESTION_MARK
			case '"', '\'':
				insertSemicolon = true
				tkn = token.STRING
				var err error
				literal, err = self.scanString(self.chrOffset - 1)
				if err != nil {
					tkn = token.ILLEGAL
				}
			default:
				self.errorUnexpected(idx, chr)
				tkn = token.ILLEGAL
			}
		}
		self.insertSemicolon = insertSemicolon
		return
	}
}

func (self *_parser) switch2(tkn0, tkn1 token.Token) token.Token {
	if self.chr == '=' {
		self.read()
		return tkn1
	}
	return tkn0
}

func (self *_parser) switch3(tkn0, tkn1 token.Token, chr2 rune, tkn2 token.Token) token.Token {
	if self.chr == '=' {
		self.read()
		return tkn1
	}
	if self.chr == chr2 {
		self.read()
		return tkn2
	}
	return tkn0
}

func (self *_parser) switch4(tkn0, tkn1 token.Token, chr2 rune, tkn2, tkn3 token.Token) token.Token {
	if self.chr == '=' {
		self.read()
		return tkn1
	}
	if self.chr == chr2 {
		self.read()
		if self.chr == '=' {
			self.read()
			return tkn3
		}
		return tkn2
	}
	return tkn0
}

func (self *_parser) switch6(tkn0, tkn1 token.Token, chr2 rune, tkn2, tkn3 token.Token, chr3 rune, tkn4, tkn5 token.Token) token.Token {
	if self.chr == '=' {
		self.read()
		return tkn1
	}
	if self.chr == chr2 {
		self.read()
		if self.chr == '=' {
			self.read()
			return tkn3
		}
		if self.chr == chr3 {
			self.read()
			if self.chr == '=' {
				self.read()
				return tkn5
			}
			return tkn4
		}
		return tkn2
	}
	return tkn0
}

func (self *_parser) chrAt(index int) _chr {
	value, width := utf8.DecodeRuneInString(self.str[index:])
	return _chr{
		value: value,
		width: width,
	}
}

func (self *_parser) _peek() rune {
	if self.offset+1 < self.length {
		return rune(self.str[self.offset+1])
	}
	return -1
}

func (self *_parser) read() {
	if self.offset < self.length {
		self.chrOffset = self.offset
		chr, width := rune(self.str[self.offset]), 1
		if chr >= utf8.RuneSelf { // !ASCII
			chr, width = utf8.DecodeRuneInString(self.str[self.offset:])
			if chr == utf8.RuneError && width == 1 {
				self.error(self.chrOffset, "Invalid UTF-8 character")
			}
		}
		self.offset += width
		self.chr = chr
	} else {
		self.chrOffset = self.length
		self.chr = -1 // EOF
	}
}

// This is here since the functions are so similar
func (self *_RegExp_parser) read() {
	if self.offset < self.length {
		self.chrOffset = self.offset
		chr, width := rune(self.str[self.offset]), 1
		if chr >= utf8.RuneSelf { // !ASCII
			chr, width = utf8.DecodeRuneInString(self.str[self.offset:])
			if chr == utf8.RuneError && width == 1 {
				self.error(self.chrOffset, "Invalid UTF-8 character")
			}
		}
		self.offset += width
		self.chr = chr
	} else {
		self.chrOffset = self.length
		self.chr = -1 // EOF
	}
}

func (self *_parser) readSingleLineComment() (result []rune) {
	for self.chr != -1 {
		self.read()
		if isLineTerminator(self.chr) {
			return
		}
		result = append(result, self.chr)
	}

	// Get rid of the trailing -1
	result = result[:len(result)-1]

	return
}

func (self *_parser) readMultiLineComment() (result []rune) {
	self.read()
	for self.chr >= 0 {
		chr := self.chr
		self.read()
		if chr == '*' && self.chr == '/' {
			self.read()
			return
		}

		result = append(result, chr)
	}

	self.errorUnexpected(0, self.chr)

	return
}

func (self *_parser) skipSingleLineComment() {
	for self.chr != -1 {
		self.read()
		if isLineTerminator(self.chr) {
			return
		}
	}
}

func (self *_parser) skipMultiLineComment() {
	self.read()
	for self.chr >= 0 {
		chr := self.chr
		self.read()
		if chr == '*' && self.chr == '/' {
			self.read()
			return
		}
	}

	self.errorUnexpected(0, self.chr)
}

func (self *_parser) skipWhiteSpace() {
	for {
		switch self.chr {
		case ' ', '\t', '\f', '\v', '\u00a0', '\ufeff':
			self.read()
			continue
		case '\r':
			if self._peek() == '\n' {
				self.comments.AtLineBreak()
				self.read()
			}
			fallthrough
		case '\u2028', '\u2029', '\n':
			if self.insertSemicolon {
				return
			}
			self.comments.AtLineBreak()
			self.read()
			continue
		}
		if self.chr >= utf8.RuneSelf {
			if unicode.IsSpace(self.chr) {
				self.read()
				continue
			}
		}
		break
	}
}

func (self *_parser) skipLineWhiteSpace() {
	for isLineWhiteSpace(self.chr) {
		self.read()
	}
}

func (self *_parser) scanMantissa(base int) {
	for digitValue(self.chr) < base {
		self.read()
	}
}

func (self *_parser) scanEscape(quote rune) {

	var length, base uint32
	switch self.chr {
	//case '0', '1', '2', '3', '4', '5', '6', '7':
	//    Octal:
	//    length, base, limit = 3, 8, 255
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', '"', '\'', '0':
		self.read()
		return
	case '\r', '\n', '\u2028', '\u2029':
		self.scanNewline()
		return
	case 'x':
		self.read()
		length, base = 2, 16
	case 'u':
		self.read()
		length, base = 4, 16
	default:
		self.read() // Always make progress
		return
	}

	var value uint32
	for ; length > 0 && self.chr != quote && self.chr >= 0; length-- {
		digit := uint32(digitValue(self.chr))
		if digit >= base {
			break
		}
		value = value*base + digit
		self.read()
	}
}

func (self *_parser) scanString(offset int) (string, error) {
	// " ' /
	quote := rune(self.str[offset])

	for self.chr != quote {
		chr := self.chr
		if chr == '\n' || chr == '\r' || chr == '\u2028' || chr == '\u2029' || chr < 0 {
			goto newline
		}
		self.read()
		if chr == '\\' {
			if quote == '/' {
				if self.chr == '\n' || self.chr == '\r' || self.chr == '\u2028' || self.chr == '\u2029' || self.chr < 0 {
					goto newline
				}
				self.read()
			} else {
				self.scanEscape(quote)
			}
		} else if chr == '[' && quote == '/' {
			// Allow a slash (/) in a bracket character class ([...])
			// TODO Fix this, this is hacky...
			quote = -1
		} else if chr == ']' && quote == -1 {
			quote = '/'
		}
	}

	// " ' /
	self.read()

	return string(self.str[offset:self.chrOffset]), nil

newline:
	self.scanNewline()
	err := "String not terminated"
	if quote == '/' {
		err = "Invalid regular expression: missing /"
		self.error(self.idxOf(offset), err)
	}
	return "", errors.New(err)
}

func (self *_parser) scanNewline() {
	if self.chr == '\r' {
		self.read()
		if self.chr != '\n' {
			return
		}
	}
	self.read()
}

func hex2decimal(chr byte) (value rune, ok bool) {
	{
		chr := rune(chr)
		switch {
		case '0' <= chr && chr <= '9':
			return chr - '0', true
		case 'a' <= chr && chr <= 'f':
			return chr - 'a' + 10, true
		case 'A' <= chr && chr <= 'F':
			return chr - 'A' + 10, true
		}
		return
	}
}

func parseNumberLiteral(literal string) (value interface{}, err error) {
	// TODO Is Uint okay? What about -MAX_UINT
	value, err = strconv.ParseInt(literal, 0, 64)
	if err == nil {
		return
	}

	parseIntErr := err // Save this first error, just in case

	value, err = strconv.ParseFloat(literal, 64)
	if err == nil {
		return
	} else if err.(*strconv.NumError).Err == strconv.ErrRange {
		// Infinity, etc.
		return value, nil
	}

	err = parseIntErr

	if err.(*strconv.NumError).Err == strconv.ErrRange {
		if len(literal) > 2 && literal[0] == '0' && (literal[1] == 'X' || literal[1] == 'x') {
			// Could just be a very large number (e.g. 0x8000000000000000)
			var value float64
			literal = literal[2:]
			for _, chr := range literal {
				digit := digitValue(chr)
				if digit >= 16 {
					goto error
				}
				value = value*16 + float64(digit)
			}
			return value, nil
		}
	}

error:
	return nil, errors.New("Illegal numeric literal")
}

func parseStringLiteral(literal string) (string, error) {
	// Best case scenario...
	if literal == "" {
		return "", nil
	}

	// Slightly less-best case scenario...
	if !strings.ContainsRune(literal, '\\') {
		return literal, nil
	}

	str := literal
	buffer := bytes.NewBuffer(make([]byte, 0, 3*len(literal)/2))

	for len(str) > 0 {
		switch chr := str[0]; {
		// We do not explicitly handle the case of the quote
		// value, which can be: " ' /
		// This assumes we're already passed a partially well-formed literal
		case chr >= utf8.RuneSelf:
			chr, size := utf8.DecodeRuneInString(str)
			buffer.WriteRune(chr)
			str = str[size:]
			continue
		case chr != '\\':
			buffer.WriteByte(chr)
			str = str[1:]
			continue
		}

		if len(str) <= 1 {
			panic("len(str) <= 1")
		}
		chr := str[1]
		var value rune
		if chr >= utf8.RuneSelf {
			str = str[1:]
			var size int
			value, size = utf8.DecodeRuneInString(str)
			str = str[size:] // \ + <character>
		} else {
			str = str[2:] // \<character>
			switch chr {
			case 'b':
				value = '\b'
			case 'f':
				value = '\f'
			case 'n':
				value = '\n'
			case 'r':
				value = '\r'
			case 't':
				value = '\t'
			case 'v':
				value = '\v'
			case 'x', 'u':
				size := 0
				switch chr {
				case 'x':
					size = 2
				case 'u':
					size = 4
				}
				if len(str) < size {
					return "", fmt.Errorf("invalid escape: \\%s: len(%q) != %d", string(chr), str, size)
				}
				for j := 0; j < size; j++ {
					decimal, ok := hex2decimal(str[j])
					if !ok {
						return "", fmt.Errorf("invalid escape: \\%s: %q", string(chr), str[:size])
					}
					value = value<<4 | decimal
				}
				str = str[size:]
				if chr == 'x' {
					break
				}
				if value > utf8.MaxRune {
					panic("value > utf8.MaxRune")
				}
			case '0':
				if len(str) == 0 || '0' > str[0] || str[0] > '7' {
					value = 0
					break
				}
				fallthrough
			case '1', '2', '3', '4', '5', '6', '7':
				// TODO strict
				value = rune(chr) - '0'
				j := 0
				for ; j < 2; j++ {
					if len(str) < j+1 {
						break
					}
					chr := str[j]
					if '0' > chr || chr > '7' {
						break
					}
					decimal := rune(str[j]) - '0'
					value = (value << 3) | decimal
				}
				str = str[j:]
			case '\\':
				value = '\\'
			case '\'', '"':
				value = rune(chr)
			case '\r':
				if len(str) > 0 {
					if str[0] == '\n' {
						str = str[1:]
					}
				}
				fallthrough
			case '\n':
				continue
			default:
				value = rune(chr)
			}
		}
		buffer.WriteRune(value)
	}

	return buffer.String(), nil
}

func (self *_parser) scanNumericLiteral(decimalPoint bool) (token.Token, string) {

	offset := self.chrOffset
	tkn := token.NUMBER

	if decimalPoint {
		offset--
		self.scanMantissa(10)
		goto exponent
	}

	if self.chr == '0' {
		offset := self.chrOffset
		self.read()
		if self.chr == 'x' || self.chr == 'X' {
			// Hexadecimal
			self.read()
			if isDigit(self.chr, 16) {
				self.read()
			} else {
				return token.ILLEGAL, self.str[offset:self.chrOffset]
			}
			self.scanMantissa(16)

			if self.chrOffset-offset <= 2 {
				// Only "0x" or "0X"
				self.error(0, "Illegal hexadecimal number")
			}

			goto hexadecimal
		} else if self.chr == '.' {
			// Float
			goto float
		} else {
			// Octal, Float
			if self.chr == 'e' || self.chr == 'E' {
				goto exponent
			}
			self.scanMantissa(8)
			if self.chr == '8' || self.chr == '9' {
				return token.ILLEGAL, self.str[offset:self.chrOffset]
			}
			goto octal
		}
	}

	self.scanMantissa(10)

float:
	if self.chr == '.' {
		self.read()
		self.scanMantissa(10)
	}

exponent:
	if self.chr == 'e' || self.chr == 'E' {
		self.read()
		if self.chr == '-' || self.chr == '+' {
			self.read()
		}
		if isDecimalDigit(self.chr) {
			self.read()
			self.scanMantissa(10)
		} else {
			return token.ILLEGAL, self.str[offset:self.chrOffset]
		}
	}

hexadecimal:
octal:
	if isIdentifierStart(self.chr) || isDecimalDigit(self.chr) {
		return token.ILLEGAL, self.str[offset:self.chrOffset]
	}

	return tkn, self.str[offset:self.chrOffset]
}
