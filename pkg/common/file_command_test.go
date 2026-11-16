package common

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// utf16le encodes s as UTF-16LE, optionally with a leading BOM.
func utf16le(s string, bom bool) []byte {
	var b []byte
	if bom {
		b = append(b, 0xFF, 0xFE)
	}
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

// TestNewFileCommandReader checks that BOM-marked UTF-8 and UTF-16 decode to
// UTF-8 and that content without a BOM passes through unchanged.
func TestNewFileCommandReader(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"plain utf8", []byte("FOO=bar\n"), "FOO=bar\n"},
		{"utf8 bom", append([]byte{0xEF, 0xBB, 0xBF}, "FOO=bär\n"...), "FOO=bär\n"},
		{"utf16le bom", utf16le("C:\\cygwin\\bin\r\n", true), "C:\\cygwin\\bin\r\n"},
		{"utf16be bom", []byte{0xFE, 0xFF, 0x00, 'A', 0x00, '\n'}, "A\n"},
		{"non-utf8 without bom is untouched", []byte{'A', 0xE9, '\n'}, "A\xe9\n"},
		{"empty", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := io.ReadAll(NewFileCommandReader(bytes.NewReader(tt.input)))
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}
