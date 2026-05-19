package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/vpn"
)

func TestRunVPNHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"vpn", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(vpn --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope vpn [--json] [--last 60m] [vpn-name]") {
		t.Fatalf("stdout = %q, want vpn usage", stdout.String())
	}
}

func TestParseVPNFlags(t *testing.T) {
	flags, err := parseVPNFlags([]string{"--json", "--last", "10m", "Work VPN"})
	if err != nil {
		t.Fatal(err)
	}
	if !flags.json || flags.last != "10m" || flags.name != "Work VPN" {
		t.Fatalf("flags = %#v, want json last name", flags)
	}
}

func TestRenderVPNReport(t *testing.T) {
	var buf bytes.Buffer
	report := vpn.Report{
		RequestedName:  "Work VPN",
		LogWindow:      "60m",
		SelectedStatus: "Disconnected",
		Services: []vpn.Service{
			{Name: "Work VPN", Status: "Disconnected"},
		},
		Interfaces: []vpn.Interface{
			{Name: "utun4", Status: "active", Addresses: []string{"inet 10.0.0.2"}},
		},
		Findings: []vpn.Finding{
			{Category: "VPN_SERVICE_DISCONNECTED", Severity: "medium", Confidence: 0.78, Evidence: []string{"Disconnected"}, Source: "scutil --nc status"},
		},
	}

	if err := renderVPNReport(&buf, report); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"VPN Triage:", "Work VPN", "VPN_SERVICE_DISCONNECTED", "utun4"} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}
