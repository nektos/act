package common

import (
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
type Executor func() error

// Conditional define contract for the conditional predicate
type Conditional func() bool

// NewPipelineExecutor creates a new executor from a series of other executors
func NewPipelineExecutor(executors ...Executor) Executor {
	return func() error {
		for _, executor := range executors {
			if executor == nil {
				continue
			}
			err := executor()
			if err != nil {
				switch err.(type) {
				case Warning:
					log.Warning(err.Error())
					return nil
				default:
					log.Debugf("%+v", err)
					return err
				}
			}
		}
		return nil
	}
}

// NewConditionalExecutor creates a new executor based on conditions
func NewConditionalExecutor(conditional Conditional, trueExecutor Executor, falseExecutor Executor) Executor {
	return func() error {
		if conditional() {
			if trueExecutor != nil {
				return trueExecutor()
			}
		} else {
			if falseExecutor != nil {
				return falseExecutor()
			}
		}
		return nil
	}
}

func executeWithChan(executor Executor, errChan chan error) {
	errChan <- executor()
}

// NewErrorExecutor creates a new executor that always errors out
func NewErrorExecutor(err error) Executor {
	return func() error {
		return err
	}
}

// NewParallelExecutor creates a new executor from a parallel of other executors
func NewParallelExecutor(executors ...Executor) Executor {
	return func() error {
		errChan := make(chan error)

		for _, executor := range executors {
			go executeWithChan(executor, errChan)
		}

		for i := 0; i < len(executors); i++ {
			err := <-errChan
			if err != nil {
				return err
			}
		}
		return nil
	}
}
