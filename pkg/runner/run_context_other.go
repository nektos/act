//go:build !linux

package runner

import (
	"context"
	"fmt"

	"github.com/nektos/act/pkg/common"
)

func (rc *RunContext) startLxcEnvironment() common.Executor {
	return func(_ context.Context) error {
		return fmt.Errorf("You need macOS for tart")
	}
}
