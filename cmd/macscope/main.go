package main

import (
	"context"
	"os"

	"github.com/jdefrancesco/macscope/internal/cli"
	"github.com/jdefrancesco/macscope/internal/output"
)

func main() {
	streams := output.Streams{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}

	os.Exit(cli.Run(context.Background(), os.Args[1:], streams))
}
