package runner

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeTerminal(t *testing.T) {
	for in, want := range map[string]string{
		"plain text":                  "plain text",
		"tab\there\r":                 "tab\there\r",
		"unicode ✓ é £":               "unicode ✓ é £",
		"\x1b[31mred\x1b[0m":          "\x1b[31mred\x1b[0m",
		"\x1b[1;38;5;208mbold\x1b[m":  "\x1b[1;38;5;208mbold\x1b[m",
		"\x1b]0;pwned title\x07after": "after",
		"\x1b]8;;https://evil.test\x1b\\link\x1b]8;;\x1b\\": "link",
		"\x1b]52;c;ZXZpbA==\x07clip":                        "clip",
		"\x1bP+q544e\x1b\\dcs":                              "dcs",
		"\x1b_apc\x1b\\x":                                   "x",
		"\x1b[2J\x1b[Hcleared":                              "cleared",
		"\x1b[1Aup":                                         "up",
		"\x1b[?1049hsecret":                                 "secret",
		"bell\x07 bs\x08 nul\x00":                           "bell bs nul",
		"\x1bcreset":                                        "reset",
		"\u009b2Jc1":                                        "c1",
		"\u009d0;title\u009cafter":                          "after",
		"\u0085nel":                                         "nel",
		"trailing\x1b":                                      "trailing",
		"\x1b[31":                                           "",
		"\x1b]0;unterminated":                               "",
		"del\x7f":                                           "del",
	} {
		assert.Equal(t, want, sanitizeTerminal(in), "%q", in)
	}
}

func TestJobLogFormatterStripsControlSequences(t *testing.T) {
	logger := logrus.New()
	f := &jobLogFormatter{color: blue}
	entry := logrus.NewEntry(logger).WithFields(logrus.Fields{"job": "build\x1b]0;x\x07", "raw_output": true})
	entry.Message = "out \x1b]0;pwned\x07\x1b[32mgreen\x1b[0m\n"
	out, err := f.Format(entry)
	assert.NoError(t, err)
	assert.Equal(t, "[build]   | out \x1b[32mgreen\x1b[0m\n", string(out))
	assert.False(t, bytes.Contains(out, []byte("pwned")))
}
