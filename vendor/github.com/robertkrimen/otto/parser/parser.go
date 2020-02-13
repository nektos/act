/*
Package parser implements a parser for JavaScript.

    import (
        "github.com/robertkrimen/otto/parser"
    )

Parse and return an AST

    filename := "" // A filename is optional
    src := `
        // Sample xyzzy example
        (function(){
            if (3.14159 > 0) {
                console.log("Hello, World.");
                return;
            }

            var xyzzy = NaN;
            console.log("Nothing happens.");
            return xyzzy;
        })();
    `

    // Parse some JavaScript, yielding a *ast.Program and/or an ErrorList
    program, err := parser.ParseFile(nil, filename, src, 0)

Warning

The parser and AST interfaces are still works-in-progress (particularly where
node types are concerned) and may change in the future.

*/
package parser

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"io/ioutil"

	"github.com/robertkrimen/otto/ast"
	"github.com/robertkrimen/otto/file"
	"github.com/robertkrimen/otto/token"
	"gopkg.in/sourcemap.v1"
)

// A Mode value is a set of flags (or 0). They control optional parser functionality.
type Mode uint

const (
	IgnoreRegExpErrors Mode = 1 << iota // Ignore RegExp compatibility errors (allow backtracking)
	StoreComments                       // Store the comments from source to the comments map
)

type _parser struct {
	str    string
	length int
	base   int

	chr       rune // The current character
	chrOffset int  // The offset of current character
	offset    int  // The offset after current character (may be greater than 1)

	idx     file.Idx    // The index of token
	token   token.Token // The token
	literal string      // The literal of the token, if any

	scope             *_scope
	insertSemicolon   bool // If we see a newline, then insert an implicit semicolon
	implicitSemicolon bool // An implicit semicolon exists

	errors ErrorList

	recover struct {
		// Scratch when trying to seek to the next statement, etc.
		idx   file.Idx
		count int
	}

	mode Mode

	file *file.File

	comments *ast.Comments
}

type Parser interface {
	Scan() (tkn token.Token, literal string, idx file.Idx)
}

func _newParser(filename, src string, base int, sm *sourcemap.Consumer) *_parser {
	return &_parser{
		chr:      ' ', // This is set so we can start scanning by skipping whitespace
		str:      src,
		length:   len(src),
		base:     base,
		file:     file.NewFile(filename, src, base).WithSourceMap(sm),
		comments: ast.NewComments(),
	}
}

// Returns a new Parser.
func NewParser(filename, src string) Parser {
	return _newParser(filename, src, 1, nil)
}

func ReadSource(filename string, src interface{}) ([]byte, error) {
	if src != nil {
		switch src := src.(type) {
		case string:
			return []byte(src), nil
		case []byte:
			return src, nil
		case *bytes.Buffer:
			if src != nil {
				return src.Bytes(), nil
			}
		case io.Reader:
			var bfr bytes.Buffer
			if _, err := io.Copy(&bfr, src); err != nil {
				return nil, err
			}
			return bfr.Bytes(), nil
		}
		return nil, errors.New("invalid source")
	}
	return ioutil.ReadFile(filename)
}

func ReadSourceMap(filename string, src interface{}) (*sourcemap.Consumer, error) {
	if src == nil {
		return nil, nil
	}

	switch src := src.(type) {
	case string:
		return sourcemap.Parse(filename, []byte(src))
	case []byte:
		return sourcemap.Parse(filename, src)
	case *bytes.Buffer:
		if src != nil {
			return sourcemap.Parse(filename, src.Bytes())
		}
	case io.Reader:
		var bfr bytes.Buffer
		if _, err := io.Copy(&bfr, src); err != nil {
			return nil, err
		}
		return sourcemap.Parse(filename, bfr.Bytes())
	case *sourcemap.Consumer:
		return src, nil
	}

	return nil, errors.New("invalid sourcemap type")
}

func ParseFileWithSourceMap(fileSet *file.FileSet, filename string, javascriptSource, sourcemapSource interface{}, mode Mode) (*ast.Program, error) {
	src, err := ReadSource(filename, javascriptSource)
	if err != nil {
		return nil, err
	}

	if sourcemapSource == nil {
		lines := bytes.Split(src, []byte("\n"))
		lastLine := lines[len(lines)-1]
		if bytes.HasPrefix(lastLine, []byte("//# sourceMappingURL=data:application/json")) {
			bits := bytes.SplitN(lastLine, []byte(","), 2)
			if len(bits) == 2 {
				if d, err := base64.StdEncoding.DecodeString(string(bits[1])); err == nil {
					sourcemapSource = d
				}
			}
		}
	}

	sm, err := ReadSourceMap(filename, sourcemapSource)
	if err != nil {
		return nil, err
	}

	base := 1
	if fileSet != nil {
		base = fileSet.AddFile(filename, string(src))
	}

	parser := _newParser(filename, string(src), base, sm)
	parser.mode = mode
	program, err := parser.parse()
	program.Comments = parser.comments.CommentMap

	return program, err
}

// ParseFile parses the source code of a single JavaScript/ECMAScript source file and returns
// the corresponding ast.Program node.
//
// If fileSet == nil, ParseFile parses source without a FileSet.
// If fileSet != nil, ParseFile first adds filename and src to fileSet.
//
// The filename argument is optional and is used for labelling errors, etc.
//
// src may be a string, a byte slice, a bytes.Buffer, or an io.Reader, but it MUST always be in UTF-8.
//
//      // Parse some JavaScript, yielding a *ast.Program and/or an ErrorList
//      program, err := parser.ParseFile(nil, "", `if (abc > 1) {}`, 0)
//
func ParseFile(fileSet *file.FileSet, filename string, src interface{}, mode Mode) (*ast.Program, error) {
	return ParseFileWithSourceMap(fileSet, filename, src, nil, mode)
}

// ParseFunction parses a given parameter list and body as a function and returns the
// corresponding ast.FunctionLiteral node.
//
// The parameter list, if any, should be a comma-separated list of identifiers.
//
func ParseFunction(parameterList, body string) (*ast.FunctionLiteral, error) {

	src := "(function(" + parameterList + ") {\n" + body + "\n})"

	parser := _newParser("", src, 1, nil)
	program, err := parser.parse()
	if err != nil {
		return nil, err
	}

	return program.Body[0].(*ast.ExpressionStatement).Expression.(*ast.FunctionLiteral), nil
}

// Scan reads a single token from the source at the current offset, increments the offset and
// returns the token.Token token, a string literal representing the value of the token (if applicable)
// and it's current file.Idx index.
func (self *_parser) Scan() (tkn token.Token, literal string, idx file.Idx) {
	return self.scan()
}

func (self *_parser) slice(idx0, idx1 file.Idx) string {
	from := int(idx0) - self.base
	to := int(idx1) - self.base
	if from >= 0 && to <= len(self.str) {
		return self.str[from:to]
	}

	return ""
}

func (self *_parser) parse() (*ast.Program, error) {
	self.next()
	program := self.parseProgram()
	if false {
		self.errors.Sort()
	}

	if self.mode&StoreComments != 0 {
		self.comments.CommentMap.AddComments(program, self.comments.FetchAll(), ast.TRAILING)
	}

	return program, self.errors.Err()
}

func (self *_parser) next() {
	self.token, self.literal, self.idx = self.scan()
}

func (self *_parser) optionalSemicolon() {
	if self.token == token.SEMICOLON {
		self.next()
		return
	}

	if self.implicitSemicolon {
		self.implicitSemicolon = false
		return
	}

	if self.token != token.EOF && self.token != token.RIGHT_BRACE {
		self.expect(token.SEMICOLON)
	}
}

func (self *_parser) semicolon() {
	if self.token != token.RIGHT_PARENTHESIS && self.token != token.RIGHT_BRACE {
		if self.implicitSemicolon {
			self.implicitSemicolon = false
			return
		}

		self.expect(token.SEMICOLON)
	}
}

func (self *_parser) idxOf(offset int) file.Idx {
	return file.Idx(self.base + offset)
}

func (self *_parser) expect(value token.Token) file.Idx {
	idx := self.idx
	if self.token != value {
		self.errorUnexpectedToken(self.token)
	}
	self.next()
	return idx
}

func lineCount(str string) (int, int) {
	line, last := 0, -1
	pair := false
	for index, chr := range str {
		switch chr {
		case '\r':
			line += 1
			last = index
			pair = true
			continue
		case '\n':
			if !pair {
				line += 1
			}
			last = index
		case '\u2028', '\u2029':
			line += 1
			last = index + 2
		}
		pair = false
	}
	return line, last
}

func (self *_parser) position(idx file.Idx) file.Position {
	position := file.Position{}
	offset := int(idx) - self.base
	str := self.str[:offset]
	position.Filename = self.file.Name()
	line, last := lineCount(str)
	position.Line = 1 + line
	if last >= 0 {
		position.Column = offset - last
	} else {
		position.Column = 1 + len(str)
	}

	return position
}
