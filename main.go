package main

import (
	_ "embed"

	"github.com/nektos/act/cmd"
	"github.com/nektos/act/pkg/common"
)

//go:embed VERSION
var version string

func main() {
	ctx, cancel := common.CreateGracefulJobCancellationContext()
	defer cancel()

	// run the command
	cmd.Execute(ctx, version)
}
