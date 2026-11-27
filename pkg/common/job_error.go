package common

import (
	"context"
)

type jobErrorContextKey string

const jobErrorContextKeyVal = jobErrorContextKey("job.error")

type jobCancelCtx string

const JobCancelCtxVal = jobCancelCtx("job.cancel")

type failFastContextKey string

const FailFastContextKeyVal = failFastContextKey("job.failfast")

// WithFailFast adds fail-fast configuration to the context
func WithFailFast(ctx context.Context, failFast bool) context.Context {
	return context.WithValue(ctx, FailFastContextKeyVal, failFast)
}

// IsFailFast returns whether fail-fast is enabled for this context
func IsFailFast(ctx context.Context) bool {
	val := ctx.Value(FailFastContextKeyVal)
	if val != nil {
		if ff, ok := val.(bool); ok {
			return ff
		}
	}
	return false
}

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

func WithJobCancelContext(ctx context.Context, cancelContext context.Context) context.Context {
	return context.WithValue(ctx, JobCancelCtxVal, cancelContext)
}

func JobCancelContext(ctx context.Context) context.Context {
	val := ctx.Value(JobCancelCtxVal)
	if val != nil {
		if container, ok := val.(context.Context); ok {
			return container
		}
	}
	return nil
}

// EarlyCancelContext returns a new context based on ctx that is canceled when the first of the provided contexts is canceled.
func EarlyCancelContext(ctx context.Context) (context.Context, context.CancelFunc) {
	val := JobCancelContext(ctx)
	if val != nil {
		context, cancel := context.WithCancel(ctx)
		go func() {
			defer cancel()
			select {
			case <-context.Done():
			case <-ctx.Done():
			case <-val.Done():
			}
		}()
		return context, cancel
	}
	return ctx, func() {}
}
