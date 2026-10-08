// Package tui implements the interactive Bubble Tea browser behind `macscope tui`.
package tui

import (
	"context"

	"github.com/jdefrancesco/macscope/internal/output"
)

// RunFunc matches the signature of macscope CLI command handlers.
type RunFunc func(ctx context.Context, args []string, streams output.Streams) error

// Entry describes one command the browser can run.
type Entry struct {
	// Name is the command name shown in the list, e.g. "specs".
	Name string
	// Summary is a one-line description shown for the selected entry.
	Summary string
	// Args are passed to Run before any prompted value, e.g. ["--last", "30m"].
	Args []string
	// Prompt, when non-empty, asks the user for one argument (e.g. "path")
	// that is appended to Args as a single value. Empty means run immediately.
	Prompt string
	// Run executes the command, writing human-readable output to streams.Out.
	Run RunFunc
}

// CommandLine renders the equivalent shell invocation for display only.
func (e Entry) CommandLine(extra ...string) string {
	line := "macscope " + e.Name
	for _, arg := range append(append([]string{}, e.Args...), extra...) {
		line += " " + arg
	}
	return line
}
