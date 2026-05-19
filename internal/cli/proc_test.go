package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestRunProcHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"proc", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(proc --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope proc [--json] <pid-or-name>") {
		t.Fatalf("stdout = %q, want proc usage", stdout.String())
	}
}

func TestParseProcFlags(t *testing.T) {
	flags, err := parseProcFlags([]string{"--json", "ssh"})
	if err != nil {
		t.Fatal(err)
	}
	if !flags.json || flags.query != "ssh" {
		t.Fatalf("flags = %#v, want json/query", flags)
	}
}
