package logquery

import (
	"context"
	"io"
	"os/exec"
	"time"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) (collect.Result, error)
}

func Show(ctx context.Context, last string, predicate string, runner CommandRunner) (collect.Result, error) {
	if last == "" {
		last = "30m"
	}
	if runner == nil {
		runner = collect.Runner{Timeout: 45 * time.Second}
	}
	return runner.Run(ctx, "log", "show", "--last", last, "--style", "syslog", "--info", "--debug", "--predicate", predicate)
}

func Stream(ctx context.Context, predicate string, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "log", "stream", "--style", "syslog", "--info", "--debug", "--predicate", predicate)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
