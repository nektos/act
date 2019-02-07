package parser

type OptionFunc func(*Parser)

func WithSuppressWarnings() OptionFunc {
	return func(ps *Parser) {
		ps.suppressSeverity = WARNING
	}
}

func WithSuppressErrors() OptionFunc {
	return func(ps *Parser) {
		ps.suppressSeverity = ERROR
	}
}
