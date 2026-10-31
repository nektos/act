package container

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/nektos/act/pkg/common"
	assert "github.com/stretchr/testify/assert"
)

type mockContainer struct {
	content string
	err     error
}

func (m *mockContainer) GetContainerArchive(_ context.Context, _ string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: "env",
		Size: int64(len(m.content)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write([]byte(m.content))
	_ = tw.Close()

	return io.NopCloser(&buf), nil
}

func (m *mockContainer) Create([]string, []string) common.Executor              { return nil }
func (m *mockContainer) Copy(string, ...*FileEntry) common.Executor             { return nil }
func (m *mockContainer) CopyTarStream(context.Context, string, io.Reader) error { return nil }
func (m *mockContainer) CopyDir(string, string, bool) common.Executor           { return nil }
func (m *mockContainer) Pull(bool) common.Executor                              { return nil }
func (m *mockContainer) Start(bool) common.Executor                             { return nil }
func (m *mockContainer) Exec([]string, map[string]string, string, string) common.Executor {
	return nil
}
func (m *mockContainer) UpdateFromEnv(string, *map[string]string) common.Executor { return nil }
func (m *mockContainer) UpdateFromImageEnv(*map[string]string) common.Executor    { return nil }
func (m *mockContainer) Remove() common.Executor                                  { return nil }
func (m *mockContainer) Close() common.Executor                                   { return nil }
func (m *mockContainer) ReplaceLogWriter(io.Writer, io.Writer) (io.Writer, io.Writer) {
	return nil, nil
}
func (m *mockContainer) GetHealth(context.Context) Health { return HealthHealthy }

func TestParseEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		initial  map[string]string
		expected map[string]string
	}{
		{
			name:    "single line env var",
			content: "FOO=bar",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "bar",
			},
		},
		{
			name:    "multiple single line env vars",
			content: "FOO=bar\nBAZ=qux",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		{
			name:    "value containing equals sign",
			content: "FOO=bar=baz",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "bar=baz",
			},
		},
		{
			name:    "multiline env var with heredoc delimiter",
			content: "FOO<<EOF\nline1\nline2\nEOF",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "line1\nline2",
			},
		},
		{
			name:    "continuation line appended to last key",
			content: "FOO=bar\ncontinuation line",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "bar\ncontinuation line",
			},
		},
		{
			name:    "multiple continuation lines",
			content: "FOO=first\nsecond\nthird",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "first\nsecond\nthird",
			},
		},
		{
			name:    "continuation only applies to most recent key",
			content: "FOO=one\nBAR=two\ncontinued",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "one",
				"BAR": "two\ncontinued",
			},
		},
		{
			name:    "continuation after multiline heredoc var",
			content: "FOO<<EOF\nheredoc content\nEOF\nBAR=value\nextra line",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "heredoc content",
				"BAR": "value\nextra line",
			},
		},
		{
			name:    "orphan line with no preceding key is ignored",
			content: "orphan line\nFOO=bar",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "bar",
			},
		},
		{
			name:    "empty value",
			content: "FOO=",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "",
			},
		},
		{
			name:    "preserves existing env vars",
			content: "NEW=value",
			initial: map[string]string{"EXISTING": "kept"},
			expected: map[string]string{
				"EXISTING": "kept",
				"NEW":      "value",
			},
		},
		{
			name:    "utf8 bom is stripped from first line",
			content: "\xef\xbb\xbfFOO=bar",
			initial: map[string]string{},
			expected: map[string]string{
				"FOO": "bar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockContainer{content: tt.content}
			env := tt.initial
			executor := parseEnvFile(mock, "/path/to/env", &env)
			err := executor(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, env)
		})
	}
}

func TestParseEnvFileMultilineDelimiterNotFound(t *testing.T) {
	mock := &mockContainer{content: "FOO<<EOF\nno closing delimiter"}
	env := map[string]string{}
	executor := parseEnvFile(mock, "/path/to/env", &env)
	err := executor(context.Background())
	assert.ErrorContains(t, err, "invalid format delimiter 'EOF' not found before end of file")
}

func TestParseEnvFileArchiveError(t *testing.T) {
	mock := &mockContainer{err: io.ErrUnexpectedEOF}
	env := map[string]string{}
	executor := parseEnvFile(mock, "/path/to/env", &env)
	err := executor(context.Background())
	assert.NoError(t, err, "archive errors should be silently ignored")
}
