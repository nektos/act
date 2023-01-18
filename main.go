package main

import (
	"context"
	_ "embed"
	"os"
	"os/signal"
	"syscall"

	"github.com/nektos/act/cmd"
)

//go:embed VERSION
var version string

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
