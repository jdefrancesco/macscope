package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jdefrancesco/macscope/internal/cli"
	"github.com/jdefrancesco/macscope/internal/output"
)

func main() {
	streams := output.Streams{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], streams)
	stop()
	os.Exit(code)
}
