package common

import (
	"context"
)

type jobErrorContextKey string

const jobErrorContextKeyVal = jobErrorContextKey("job.error")

// JobError returns the job error for current context if any
func JobError(ctx context.Context) error {
	val := ctx.Value(jobErrorContextKeyVal)
	if val != nil {
		if container, ok := val.(map[string]error); ok {
			return container["error"]
		}
	}
	return nil
}

func SetJobError(ctx context.Context, err error) {
	ctx.Value(jobErrorContextKeyVal).(map[string]error)["error"] = err
}

// WithJobErrorContainer adds a value to the context as a container for an error
func WithJobErrorContainer(ctx context.Context) context.Context {
	container := map[string]error{}
	return context.WithValue(ctx, jobErrorContextKeyVal, container)
}
