# registry
--
    import "github.com/robertkrimen/otto/registry"

Package registry is an expirmental package to facillitate altering the otto
runtime via import.

This interface can change at any time.

## Usage

#### func  Apply

```go
func Apply(callback func(Entry))
```

#### type Entry

```go
type Entry struct {
}
```


#### func  Register

```go
func Register(source func() string) *Entry
```

#### func (*Entry) Disable

```go
func (self *Entry) Disable()
```

#### func (*Entry) Enable

```go
func (self *Entry) Enable()
```

#### func (Entry) Source

```go
func (self Entry) Source() string
```

--
**godocdown** http://github.com/robertkrimen/godocdown
