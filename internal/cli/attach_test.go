package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestRunAttachHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"attach", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(attach --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope attach [--json] [--last 30m] <pid>") {
		t.Fatalf("stdout = %q, want attach usage", stdout.String())
	}
}

func TestParseAttachFlags(t *testing.T) {
	flags, err := parseAttachFlags([]string{"--json", "--last", "10m", "123"})
	if err != nil {
		t.Fatal(err)
	}
	if !flags.json || flags.last != "10m" || flags.pid != 123 {
		t.Fatalf("flags = %#v, want json/last/pid", flags)
	}
}
