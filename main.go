package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/nektos/act/cmd"
)

var version = "v0.2.27-dev" // Manually bump after tagging next release

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// trap Ctrl+C and call cancel on the context
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	// run the command
	cmd.Execute(ctx, version)
}
