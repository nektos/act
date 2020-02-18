/*
Package registry is an expirmental package to facillitate altering the otto runtime via import.

This interface can change at any time.
*/
package registry

var registry []*Entry = make([]*Entry, 0)

type Entry struct {
	active bool
	source func() string
}

func newEntry(source func() string) *Entry {
	return &Entry{
		active: true,
		source: source,
	}
}

func (self *Entry) Enable() {
	self.active = true
}

func (self *Entry) Disable() {
	self.active = false
}

func (self Entry) Source() string {
	return self.source()
}

func Apply(callback func(Entry)) {
	for _, entry := range registry {
		if !entry.active {
			continue
		}
		callback(*entry)
	}
}

func Register(source func() string) *Entry {
	entry := newEntry(source)
	registry = append(registry, entry)
	return entry
}
