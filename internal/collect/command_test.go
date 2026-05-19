package collect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunnerCapturesOutput(t *testing.T) {
	if os.Getenv("MACSCOPE_COLLECT_HELPER") == "capture" {
		fmt.Fprint(os.Stdout, "stdout text")
		fmt.Fprint(os.Stderr, "stderr text")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	result, err := Runner{
		Timeout: 5 * time.Second,
		Env:     append(os.Environ(), "MACSCOPE_COLLECT_HELPER=capture"),
	}.Run(context.Background(), exe, "-test.run=TestRunnerCapturesOutput")
	if err != nil {
		t.Fatalf("Runner.Run returned error: %v", err)
	}

	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "stdout text") {
		t.Fatalf("Stdout = %q, want helper stdout", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "stderr text") {
		t.Fatalf("Stderr = %q, want helper stderr", result.Stderr)
	}
}

func TestRunnerReturnsStructuredExitError(t *testing.T) {
	if os.Getenv("MACSCOPE_COLLECT_HELPER") == "exit" {
		fmt.Fprint(os.Stderr, "intentional failure")
		os.Exit(7)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	result, err := Runner{
		Timeout: 5 * time.Second,
		Env:     append(os.Environ(), "MACSCOPE_COLLECT_HELPER=exit"),
	}.Run(context.Background(), exe, "-test.run=TestRunnerReturnsStructuredExitError")
	if err == nil {
		t.Fatal("Runner.Run error = nil, want CommandError")
	}

	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Runner.Run error type = %T, want *CommandError", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", result.ExitCode)
	}
	if commandErr.Result.ExitCode != 7 {
		t.Fatalf("CommandError ExitCode = %d, want 7", commandErr.Result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "intentional failure") {
		t.Fatalf("Stderr = %q, want helper stderr", result.Stderr)
	}
}
