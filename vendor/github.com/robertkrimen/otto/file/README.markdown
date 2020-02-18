# file
--
    import "github.com/robertkrimen/otto/file"

Package file encapsulates the file abstractions used by the ast & parser.

## Usage

#### type File

```go
type File struct {
}
```


#### func  NewFile

```go
func NewFile(filename, src string, base int) *File
```

#### func (*File) Base

```go
func (fl *File) Base() int
```

#### func (*File) Name

```go
func (fl *File) Name() string
```

#### func (*File) Source

```go
func (fl *File) Source() string
```

#### type FileSet

```go
type FileSet struct {
}
```

A FileSet represents a set of source files.

#### func (*FileSet) AddFile

```go
func (self *FileSet) AddFile(filename, src string) int
```
AddFile adds a new file with the given filename and src.

This an internal method, but exported for cross-package use.

#### func (*FileSet) File

```go
func (self *FileSet) File(idx Idx) *File
```

#### func (*FileSet) Position

```go
func (self *FileSet) Position(idx Idx) *Position
```
Position converts an Idx in the FileSet into a Position.

#### type Idx

```go
type Idx int
```

Idx is a compact encoding of a source position within a file set. It can be
converted into a Position for a more convenient, but much larger,
representation.

#### type Position

```go
type Position struct {
	Filename string // The filename where the error occurred, if any
	Offset   int    // The src offset
	Line     int    // The line number, starting at 1
	Column   int    // The column number, starting at 1 (The character count)

}
```

Position describes an arbitrary source position including the filename, line,
and column location.

#### func (*Position) String

```go
func (self *Position) String() string
```
String returns a string in one of several forms:

    file:line:column    A valid position with filename
    line:column         A valid position without filename
    file                An invalid position with filename
    -                   An invalid position without filename

--
**godocdown** http://github.com/robertkrimen/godocdown
