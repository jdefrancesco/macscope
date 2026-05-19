package vpn

import "testing"

func TestParseServices(t *testing.T) {
	services := ParseServices(`* (Connected) Work VPN "com.apple.ppp.l2tp"
  (Disconnected) Lab VPN "com.apple.ppp.l2tp"`)

	if len(services) != 2 {
		t.Fatalf("services length = %d, want 2", len(services))
	}
	if !services[0].Connected || services[0].Status != "Connected" {
		t.Fatalf("first service = %#v, want connected", services[0])
	}
}

func TestParseInterfaces(t *testing.T) {
	interfaces := ParseInterfaces(`en0: flags=8863<UP,BROADCAST>
	status: active
utun4: flags=8051<UP,POINTOPOINT,RUNNING,MULTICAST>
	inet 10.0.0.2 --> 10.0.0.2 netmask 0xffffffff
	status: active`)

	if len(interfaces) != 1 {
		t.Fatalf("interfaces length = %d, want 1", len(interfaces))
	}
	if interfaces[0].Name != "utun4" || interfaces[0].Status != "active" {
		t.Fatalf("interface = %#v", interfaces[0])
	}
}

func TestClassifyVPN(t *testing.T) {
	report := Report{
		RequestedName:  "Work VPN",
		SelectedStatus: "Disconnected",
		RecentLogLines: []string{
			"vpnagent failed to reconnect utun4 after disconnect",
		},
	}

	findings := Classify(report)
	if !hasFinding(findings, "VPN_SERVICE_DISCONNECTED") {
		t.Fatalf("findings = %#v, want VPN_SERVICE_DISCONNECTED", findings)
	}
	if !hasFinding(findings, "VPN_LOG_ERROR_OR_DISCONNECT") {
		t.Fatalf("findings = %#v, want VPN_LOG_ERROR_OR_DISCONNECT", findings)
	}
}

func hasFinding(findings []Finding, category string) bool {
	for _, finding := range findings {
		if finding.Category == category {
			return true
		}
	}
	return false
}
