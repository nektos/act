# parser
--
    import "github.com/robertkrimen/otto/parser"

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


### Warning

The parser and AST interfaces are still works-in-progress (particularly where
node types are concerned) and may change in the future.

## Usage

#### func  ParseFile

```go
func ParseFile(fileSet *file.FileSet, filename string, src interface{}, mode Mode) (*ast.Program, error)
```
ParseFile parses the source code of a single JavaScript/ECMAScript source file
and returns the corresponding ast.Program node.

If fileSet == nil, ParseFile parses source without a FileSet. If fileSet != nil,
ParseFile first adds filename and src to fileSet.

The filename argument is optional and is used for labelling errors, etc.

src may be a string, a byte slice, a bytes.Buffer, or an io.Reader, but it MUST
always be in UTF-8.

    // Parse some JavaScript, yielding a *ast.Program and/or an ErrorList
    program, err := parser.ParseFile(nil, "", `if (abc > 1) {}`, 0)

#### func  ParseFunction

```go
func ParseFunction(parameterList, body string) (*ast.FunctionLiteral, error)
```
ParseFunction parses a given parameter list and body as a function and returns
the corresponding ast.FunctionLiteral node.

The parameter list, if any, should be a comma-separated list of identifiers.

#### func  ReadSource

```go
func ReadSource(filename string, src interface{}) ([]byte, error)
```

#### func  TransformRegExp

```go
func TransformRegExp(pattern string) (string, error)
```
TransformRegExp transforms a JavaScript pattern into a Go "regexp" pattern.

re2 (Go) cannot do backtracking, so the presence of a lookahead (?=) (?!) or
backreference (\1, \2, ...) will cause an error.

re2 (Go) has a different definition for \s: [\t\n\f\r ]. The JavaScript
definition, on the other hand, also includes \v, Unicode "Separator, Space",
etc.

If the pattern is invalid (not valid even in JavaScript), then this function
returns the empty string and an error.

If the pattern is valid, but incompatible (contains a lookahead or
backreference), then this function returns the transformation (a non-empty
string) AND an error.

#### type Error

```go
type Error struct {
	Position file.Position
	Message  string
}
```

An Error represents a parsing error. It includes the position where the error
occurred and a message/description.

#### func (Error) Error

```go
func (self Error) Error() string
```

#### type ErrorList

```go
type ErrorList []*Error
```

ErrorList is a list of *Errors.

#### func (*ErrorList) Add

```go
func (self *ErrorList) Add(position file.Position, msg string)
```
Add adds an Error with given position and message to an ErrorList.

#### func (ErrorList) Err

```go
func (self ErrorList) Err() error
```
Err returns an error equivalent to this ErrorList. If the list is empty, Err
returns nil.

#### func (ErrorList) Error

```go
func (self ErrorList) Error() string
```
Error implements the Error interface.

#### func (ErrorList) Len

```go
func (self ErrorList) Len() int
```

#### func (ErrorList) Less

```go
func (self ErrorList) Less(i, j int) bool
```

#### func (*ErrorList) Reset

```go
func (self *ErrorList) Reset()
```
Reset resets an ErrorList to no errors.

#### func (ErrorList) Sort

```go
func (self ErrorList) Sort()
```

#### func (ErrorList) Swap

```go
func (self ErrorList) Swap(i, j int)
```

#### type Mode

```go
type Mode uint
```

A Mode value is a set of flags (or 0). They control optional parser
functionality.

```go
const (
	IgnoreRegExpErrors Mode = 1 << iota // Ignore RegExp compatibility errors (allow backtracking)
)
```

--
**godocdown** http://github.com/robertkrimen/godocdown
