package container

import (
	"context"
	"io"
)

type logWriterContextKey struct{}

type logWriters struct {
	stdout io.Writer
	stderr io.Writer
}

// WithLogWriters attaches the log writers for command output to the context.
// Execution environments prefer these writers over their global log writer,
// so steps running concurrently each capture their own output instead of
// racing for the single writer slot of the environment.
func WithLogWriters(ctx context.Context, stdout io.Writer, stderr io.Writer) context.Context {
	return context.WithValue(ctx, logWriterContextKey{}, &logWriters{stdout: stdout, stderr: stderr})
}

// LogWriters returns the log writers attached to the context, if any
func LogWriters(ctx context.Context) (io.Writer, io.Writer, bool) {
	if writers, ok := ctx.Value(logWriterContextKey{}).(*logWriters); ok {
		return writers.stdout, writers.stderr, true
	}
	return nil, nil, false
}
