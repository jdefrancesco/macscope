package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("Run(help) stderr = %q, want empty", stderr.String())
	}
	for _, want := range []string{
		"macscope <command> [flags]",
		"macscope macho [--json] [--full] <path>",
		"macscope panic --last",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("Run(help) output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"bogus"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 2 {
		t.Fatalf("Run(unknown) exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("Run(unknown) stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown command: bogus") {
		t.Fatalf("Run(unknown) stderr = %q, want unknown command", stderr.String())
	}
}

func TestRunRecognizedUnimplementedCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"panic", "--json"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 2 {
		t.Fatalf("Run(unimplemented) exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("Run(unimplemented) stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		`"panic" is recognized`,
		"Milestone 3: panic",
		"./macscope.zsh panic --json",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("Run(unimplemented) stderr missing %q:\n%s", want, stderr.String())
		}
	}
}
