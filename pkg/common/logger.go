package common

import (
	"context"

	"github.com/sirupsen/logrus"
)

type loggerContextKey string

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

// WithLogger adds a value to the context for the logger
func WithLogger(ctx context.Context, logger logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loggerContextKeyVal, logger)
}
