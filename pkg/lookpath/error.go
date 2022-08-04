package lookpath

type Error struct {
	Name string
	Err  error
}

func (e *Error) Error() string {
	return e.Err.Error()
}
