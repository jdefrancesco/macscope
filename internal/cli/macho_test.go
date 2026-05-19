package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/codesign"
	"github.com/jdefrancesco/macscope/internal/gatekeeper"
	machoreport "github.com/jdefrancesco/macscope/internal/macho"
	"github.com/jdefrancesco/macscope/internal/output"
)

func TestRunMachoHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"macho", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(macho --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope macho [--json] [--full] <path>") {
		t.Fatalf("stdout = %q, want macho usage", stdout.String())
	}
}

func TestParseMachoFlags(t *testing.T) {
	flags, err := parseMachoFlags([]string{"--json", "--full", "/bin/ls"})
	if err != nil {
		t.Fatal(err)
	}
	if !flags.json || !flags.full || flags.path != "/bin/ls" {
		t.Fatalf("flags = %#v, want json/full/path", flags)
	}
}

func TestRenderMachoReport(t *testing.T) {
	var buf bytes.Buffer
	report := machoreport.Report{
		InputPath:       "/bin/ls",
		BinaryPath:      "/bin/ls",
		SizeBytes:       123,
		SHA256:          "abc123",
		FileType:        "/bin/ls: Mach-O 64-bit executable arm64",
		Architectures:   []string{"arm64"},
		LinkedLibraries: []string{"/usr/lib/libSystem.B.dylib"},
		CodeSignature: codesign.Details{
			Identifier:     "com.apple.ls",
			TeamIdentifier: "not set",
			Authorities:    []string{"Software Signing"},
		},
		CodeSignatureVerify: codesign.Verification{
			Valid:   true,
			Message: "/bin/ls: valid on disk",
		},
		GatekeeperAssessment: gatekeeper.Assessment{
			Accepted: true,
			Source:   "Apple System",
			Raw:      "/bin/ls: accepted\nsource=Apple System",
		},
	}

	if err := renderMachoReport(&buf, report); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	for _, want := range []string{
		"Target:",
		"Signature / Policy:",
		"accepted - source=Apple System",
		"/usr/lib/libSystem.B.dylib",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("render output missing %q:\n%s", want, got)
		}
	}
}
