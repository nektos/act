package main

import (
	_ "embed"

	"github.com/harness/nektos-act/v2/cmd"
	"github.com/harness/nektos-act/v2/pkg/common"
)

//go:embed VERSION
var version string

func main() {
	ctx, cancel := common.CreateGracefulJobCancellationContext()
	defer cancel()

	// run the command
	cmd.Execute(ctx, version)
}
