package otto

// _scope:
// entryFile
// entryIdx
// top?
// outer => nil

// _stash:
// lexical
// variable
//
// _thisStash (ObjectEnvironment)
// _fnStash
// _dclStash

// An ECMA-262 ExecutionContext
type _scope struct {
	lexical  _stash
	variable _stash
	this     *_object
	eval     bool // Replace this with kind?
	outer    *_scope
	depth    int

	frame _frame
}

func newScope(lexical _stash, variable _stash, this *_object) *_scope {
	return &_scope{
		lexical:  lexical,
		variable: variable,
		this:     this,
	}
}
