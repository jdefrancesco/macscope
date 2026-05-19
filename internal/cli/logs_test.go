package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/endpointsecurity"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/tcc"
)

func TestRunTCCHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"tcc", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(tcc --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope tcc [--json] [--last 30m]") {
		t.Fatalf("stdout = %q, want tcc usage", stdout.String())
	}
}

func TestRunESHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"es", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(es --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope es [--json] [--last 30m]") {
		t.Fatalf("stdout = %q, want es usage", stdout.String())
	}
}

func TestParseLogFlags(t *testing.T) {
	flags, err := parseLogFlags("tcc", []string{"--json", "--last", "10m"})
	if err != nil {
		t.Fatal(err)
	}
	if !flags.json || flags.last != "10m" {
		t.Fatalf("flags = %#v, want json last=10m", flags)
	}
}

func TestRenderTCCReport(t *testing.T) {
	var buf bytes.Buffer
	report := tcc.Report{
		Window: "30m",
		Events: []tcc.Event{
			{Message: "deny kTCCServiceDeveloperTool client=com.example.Terminal", Service: "kTCCServiceDeveloperTool"},
		},
		Findings: []tcc.Finding{
			{Category: "TCC_DENIAL", Severity: "medium", Confidence: 0.82, Evidence: []string{"deny kTCCServiceDeveloperTool"}},
		},
	}

	if err := renderTCCReport(&buf, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "TCC_DENIAL") {
		t.Fatalf("output = %q, want TCC_DENIAL", buf.String())
	}
}

func TestRenderESReport(t *testing.T) {
	var buf bytes.Buffer
	report := endpointsecurity.Report{
		Window: "30m",
		Events: []endpointsecurity.Event{
			{Message: "EndpointSecurity denied client missing entitlement"},
		},
		Findings: []endpointsecurity.Finding{
			{Category: "ENDPOINTSECURITY_DENIAL", Severity: "medium", Confidence: 0.82, Evidence: []string{"denied"}},
		},
	}

	if err := renderESReport(&buf, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ENDPOINTSECURITY_DENIAL") {
		t.Fatalf("output = %q, want ENDPOINTSECURITY_DENIAL", buf.String())
	}
}
