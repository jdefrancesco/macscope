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
	if !strings.Contains(stdout.String(), "macscope macho [--json] [--full] [--triage] <path>") {
		t.Fatalf("stdout = %q, want macho usage", stdout.String())
	}
}

func TestParseMachoFlags(t *testing.T) {
	flags, err := parseMachoFlags([]string{"--json", "--full", "--triage", "/bin/ls"})
	if err != nil {
		t.Fatal(err)
	}
	if !flags.json || !flags.full || !flags.triage || flags.path != "/bin/ls" {
		t.Fatalf("flags = %#v, want json/full/triage/path", flags)
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

func TestRenderMachoTriageReport(t *testing.T) {
	var buf bytes.Buffer
	report := machoreport.Report{
		InputPath:     "/tmp/tool",
		BinaryPath:    "/tmp/tool",
		SizeBytes:     123,
		SHA256:        "abc123",
		FileType:      "/tmp/tool: Mach-O 64-bit executable arm64",
		Architectures: []string{"arm64"},
		CodeSignature: codesign.Details{
			Identifier: "com.example.tool",
		},
		CodeSignatureVerify: codesign.Verification{
			Valid:   false,
			Message: "/tmp/tool: code object is not signed at all",
		},
		Triage: machoreport.Triage{
			Score:   40,
			Level:   "MODERATE",
			Summary: "moderate triage score 40 from 2 evidence-backed signal(s).",
			Signals: []machoreport.TriageSignal{
				{Category: "UNSIGNED_BINARY", Points: 30, Evidence: "codesign reported unsigned binary"},
				{Category: "USER_WRITABLE_LOCATION", Points: 10, Evidence: "target path is under /tmp: /tmp/tool"},
			},
			RecommendedActions: []string{"review signing and Gatekeeper evidence before execution"},
		},
	}

	if err := renderMachoTriageReport(&buf, report); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	for _, want := range []string{"File Triage:", "40/100", "File Specifics:", "UNSIGNED_BINARY +30"} {
		if !strings.Contains(got, want) {
			t.Fatalf("triage output missing %q:\n%s", want, got)
		}
	}
}
