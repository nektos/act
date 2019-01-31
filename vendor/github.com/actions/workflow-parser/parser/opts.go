package parser

type OptionFunc func(*parseState)

func WithSuppressWarnings() OptionFunc {
	return func(ps *parseState) {
		ps.suppressSeverity = WARNING
	}
}

func WithSuppressErrors() OptionFunc {
	return func(ps *parseState) {
		ps.suppressSeverity = ERROR
	}
}
