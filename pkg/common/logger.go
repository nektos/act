package common

import (
	"context"

	"github.com/sirupsen/logrus"
)

type outputLoggerContextKey string
type loggerContextKey string

const outputLoggerContextKeyVal = outputLoggerContextKey("logrus.FieldLogger")
const loggerContextKeyVal = loggerContextKey("logrus.FieldLogger")

// Logger returns the appropriate logger for current context
func Logger(ctx context.Context) logrus.FieldLogger {
	val := ctx.Value(loggerContextKeyVal)
	if val != nil {
		if logger, ok := val.(logrus.FieldLogger); ok {
			return logger
		}
	}
	return logrus.StandardLogger()
}

// Logger returns the appropriate logger for current context
func OutputLogger(ctx context.Context) logrus.FieldLogger {
	val := ctx.Value(outputLoggerContextKeyVal)
	if val != nil {
		if logger, ok := val.(logrus.FieldLogger); ok {
			return logger
		}
	}
	return logrus.StandardLogger()
}

// WithLogger adds a value to the context for the logger
func WithLogger(ctx context.Context, logger logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loggerContextKeyVal, logger)
}

// WithOutputLogger adds a value to the context for the logger
func WithOutputLogger(ctx context.Context, logger logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, outputLoggerContextKeyVal, logger)
}
