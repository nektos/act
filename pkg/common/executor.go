package common

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// Warning that implements `error` but safe to ignore
type Warning struct {
	Message string
}

// Error the contract for error
func (w Warning) Error() string {
	return w.Message
}

// Warningf create a warning
func Warningf(format string, args ...interface{}) Warning {
	w := Warning{
		Message: fmt.Sprintf(format, args...),
	}
	return w
}

// Executor define contract for the steps of a workflow
type Executor func(ctx context.Context) error

// Conditional define contract for the conditional predicate
type Conditional func(ctx context.Context) bool

// NewInfoExecutor is an executor that logs messages
func NewInfoExecutor(format string, args ...interface{}) Executor {
	return func(ctx context.Context) error {
		logger := Logger(ctx)
		logger.Infof(format, args...)
		return nil
	}
}

// NewDebugExecutor is an executor that logs messages
func NewDebugExecutor(format string, args ...interface{}) Executor {
	return func(ctx context.Context) error {
		logger := Logger(ctx)
		logger.Debugf(format, args...)
		return nil
	}
}

// NewPipelineExecutor creates a new executor from a series of other executors
func NewPipelineExecutor(executors ...Executor) Executor {
	if len(executors) == 0 {
		return func(_ context.Context) error {
			return nil
		}
	}
	var rtn Executor
	for _, executor := range executors {
		if rtn == nil {
			rtn = executor
		} else {
			rtn = rtn.Then(executor)
		}
	}
	return rtn
}

// NewConditionalExecutor creates a new executor based on conditions
func NewConditionalExecutor(conditional Conditional, trueExecutor Executor, falseExecutor Executor) Executor {
	return func(ctx context.Context) error {
		if conditional(ctx) {
			if trueExecutor != nil {
				return trueExecutor(ctx)
			}
		} else {
			if falseExecutor != nil {
				return falseExecutor(ctx)
			}
		}
		return nil
	}
}

// NewErrorExecutor creates a new executor that always errors out
func NewErrorExecutor(err error) Executor {
	return func(_ context.Context) error {
		return err
	}
}

// NewParallelExecutor creates a new executor from a parallel of other executors
func NewParallelExecutor(parallel int, executors ...Executor) Executor {
	return func(ctx context.Context) error {
		work := make(chan Executor, len(executors))
		errs := make(chan error, len(executors))

		if 1 > parallel {
			log.Debugf("Parallel tasks (%d) below minimum, setting to 1", parallel)
			parallel = 1
		}

		for i := 0; i < parallel; i++ {
			go func(work <-chan Executor, errs chan<- error) {
				for executor := range work {
					errs <- executor(ctx)
				}
			}(work, errs)
		}

		for i := 0; i < len(executors); i++ {
			work <- executors[i]
		}
		close(work)

		// Executor waits all executors to cleanup these resources.
		var firstErr error
		for i := 0; i < len(executors); i++ {
			err := <-errs
			if firstErr == nil {
				firstErr = err
			}
		}

		if err := ctx.Err(); err != nil {
			return err
		}
		return firstErr
	}
}

// Then runs another executor if this executor succeeds
func (e Executor) Then(then Executor) Executor {
	return func(ctx context.Context) error {
		err := e(ctx)
		if err != nil {
			switch err.(type) {
			case Warning:
				Logger(ctx).Warning(err.Error())
			default:
				return err
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return then(ctx)
	}
}

// If only runs this executor if conditional is true
func (e Executor) If(conditional Conditional) Executor {
	return func(ctx context.Context) error {
		if conditional(ctx) {
			return e(ctx)
		}
		return nil
	}
}

// IfNot only runs this executor if conditional is true
func (e Executor) IfNot(conditional Conditional) Executor {
	return func(ctx context.Context) error {
		if !conditional(ctx) {
			return e(ctx)
		}
		return nil
	}
}

// IfBool only runs this executor if conditional is true
func (e Executor) IfBool(conditional bool) Executor {
	return e.If(func(_ context.Context) bool {
		return conditional
	})
}

// Finally adds an executor to run after other executor
func (e Executor) Finally(finally Executor) Executor {
	return func(ctx context.Context) error {
		err := e(ctx)
		err2 := finally(ctx)
		if err2 != nil {
			return fmt.Errorf("Error occurred running finally: %v (original error: %v)", err2, err)
		}
		return err
	}
}

// Not return an inverted conditional
func (c Conditional) Not() Conditional {
	return func(ctx context.Context) bool {
		return !c(ctx)
	}
}
