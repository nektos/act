package common

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func createGracefulJobCancellationContext() (context.Context, func(), chan os.Signal) {
	ctx := context.Background()
	ctx, forceCancel := context.WithCancel(ctx)
	cancelCtx, cancel := context.WithCancel(ctx)
	ctx = WithJobCancelContext(ctx, cancelCtx)

	// trap Ctrl+C and call cancel on the context
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-c:
			if sig == os.Interrupt {
				cancel()
				select {
				case <-c:
					forceCancel()
				case <-ctx.Done():
				}
			} else {
				forceCancel()
			}
		case <-ctx.Done():
		}
	}()
	return ctx, func() {
		signal.Stop(c)
		forceCancel()
		cancel()
	}, c
}

func CreateGracefulJobCancellationContext() (context.Context, func()) {
	ctx, cancel, _ := createGracefulJobCancellationContext()
	return ctx, cancel
}
