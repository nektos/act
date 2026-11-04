package common

import (
	"io"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// NewFileCommandReader wraps a reader over a file command (GITHUB_ENV,
// GITHUB_PATH, GITHUB_OUTPUT, ...) so that a leading byte order mark selects
// the encoding, as actions/runner does. Windows PowerShell 5.1 writes
// UTF-16LE with a BOM for `>>`, and legacy cmdlets write a UTF-8 BOM. Content
// without a BOM is passed through unchanged.
func NewFileCommandReader(r io.Reader) io.Reader {
	return transform.NewReader(r, unicode.BOMOverride(transform.Nop))
}
