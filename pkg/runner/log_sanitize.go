package runner

import (
	"strings"
	"unicode/utf8"
)

// sanitizeTerminal removes control sequences that can drive the terminal
// from step output: OSC (titles, hyperlinks, clipboard writes), DCS, APC, PM,
// SOS, cursor and erase sequences, and other C0 and C1 controls. SGR color
// sequences (ESC [ ... m), tab, newline, and carriage return are kept.
func sanitizeTerminal(s string) string {
	if !needsSanitize(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == 0x1b:
			i = skipEscape(s, i, &b)
		case c == '\t' || c == '\n' || c == '\r':
			b.WriteByte(c)
			i++
		case c < 0x20 || c == 0x7f:
			i++
		case c < utf8.RuneSelf:
			b.WriteByte(c)
			i++
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == 0x9b || r == 0x9d || r == 0x90 || r == 0x9e || r == 0x9f || r == 0x98 {
				i = skipC1String(s, i+size, r == 0x9b)
				continue
			}
			if r >= 0x80 && r <= 0x9f {
				i += size
				continue
			}
			b.WriteString(s[i : i+size])
			i += size
		}
	}
	return b.String()
}

func needsSanitize(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 0x20 && c != '\t' && c != '\n' && c != '\r') || c == 0x7f || c == 0xc2 {
			return true
		}
	}
	return false
}

// skipEscape handles the escape sequence that starts at s[i] and returns the
// index after it. SGR sequences are copied to b.
func skipEscape(s string, i int, b *strings.Builder) int {
	if i+1 >= len(s) {
		return len(s)
	}
	switch s[i+1] {
	case '[':
		end, final := scanCSI(s, i+2)
		if final == 'm' && isSGRParams(s[i+2:end-1]) {
			b.WriteString(s[i:end])
		}
		return end
	case ']', 'P', '_', '^', 'X':
		return skipString(s, i+2)
	default:
		return i + 2
	}
}

// scanCSI returns the index after a CSI sequence whose parameters start at i,
// and its final byte (0 when the sequence is unterminated).
func scanCSI(s string, i int) (int, byte) {
	for ; i < len(s); i++ {
		c := s[i]
		if c >= 0x40 && c <= 0x7e {
			return i + 1, c
		}
		if c < 0x20 || c > 0x7e {
			return i, 0
		}
	}
	return len(s), 0
}

func isSGRParams(p string) bool {
	for i := 0; i < len(p); i++ {
		c := p[i]
		if (c < '0' || c > '9') && c != ';' && c != ':' {
			return false
		}
	}
	return true
}

// skipString skips a control string up to BEL or ST (ESC \ or U+009C).
func skipString(s string, i int) int {
	for ; i < len(s); i++ {
		switch {
		case s[i] == 0x07:
			return i + 1
		case s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\':
			return i + 2
		case s[i] == 0xc2 && i+1 < len(s) && s[i+1] == 0x9c:
			return i + 2
		}
	}
	return len(s)
}

func skipC1String(s string, i int, csi bool) int {
	if csi {
		end, _ := scanCSI(s, i)
		return end
	}
	return skipString(s, i)
}
