package endpointsecurity

import "testing"

func TestParseEndpointSecurityDenial(t *testing.T) {
	input := `2026-05-19 15:11:00.000000-0400 localhost sysextd[123]: EndpointSecurity denied client missing com.apple.developer.endpoint-security.client entitlement for /dev/es`

	report := Parse("30m", input)
	if len(report.Events) != 1 {
		t.Fatalf("Events length = %d, want 1", len(report.Events))
	}
	if len(report.Findings) != 1 || report.Findings[0].Category != "ENDPOINTSECURITY_DENIAL" {
		t.Fatalf("Findings = %#v, want ENDPOINTSECURITY_DENIAL", report.Findings)
	}
}
