package tcc

import "testing"

func TestParseTCCDenial(t *testing.T) {
	input := `2026-05-19 15:11:00.000000-0400 localhost tccd[123]: [com.apple.TCC:access] deny kTCCServiceDeveloperTool client=com.example.Terminal`

	report := Parse("30m", input)
	if len(report.Events) != 1 {
		t.Fatalf("Events length = %d, want 1", len(report.Events))
	}
	if len(report.Findings) != 1 || report.Findings[0].Category != "TCC_DENIAL" {
		t.Fatalf("Findings = %#v, want TCC_DENIAL", report.Findings)
	}
	if report.Events[0].Service != "kTCCServiceDeveloperTool" {
		t.Fatalf("Service = %q", report.Events[0].Service)
	}
}
