package pmset

import (
	"testing"
	"time"
)

const sampleLog = "" +
	"2026-05-30 14:08:54 -0400 Assertions          \tPID 524(mDNSResponder) Created MaintenanceWake\n" +
	"2026-05-30 14:08:54 -0400 DarkWake            \tDarkWake from Deep Idle [CDNP] : due to NUB.SPMI0Sw3IRQ Using BATT (Charge:71%) 2 secs\n" +
	"2026-05-30 14:08:56 -0400 Sleep               \tEntering Sleep state due to 'Sleep Service Back to Sleep':TCPKeepAlive=active Using Batt (Charge:71%) 1228 secs\n" +
	"2026-05-30 14:29:24 -0400 Wake                \tWake from Standby due to EC.LidOpen/Lid Open: Using AC (Charge:100%)\n" +
	"2026-05-30 14:29:25 -0400 Wake Requests       \t[process=dasd request=SleepService deltaSecs=1567]\n" +
	"garbage line without timestamp\n" +
	"\n"

func TestParseLog(t *testing.T) {
	events := ParseLog(sampleLog)
	if len(events) != 5 {
		t.Fatalf("got %d events, want 5", len(events))
	}

	tests := []struct {
		idx         int
		wantType    string
		wantDur     int
		msgContains string
	}{
		{0, "Assertions", 0, "MaintenanceWake"},
		{1, "DarkWake", 2, "Deep Idle"},
		{2, "Sleep", 1228, "Sleep Service Back to Sleep"},
		{3, "Wake", 0, "Wake from Standby"},
		{4, "Wake Requests", 0, "process=dasd"},
	}

	for _, tt := range tests {
		event := events[tt.idx]
		if event.Type != tt.wantType {
			t.Errorf("[%d] type = %q, want %q", tt.idx, event.Type, tt.wantType)
		}
		if event.DurationSec != tt.wantDur {
			t.Errorf("[%d] duration = %d, want %d", tt.idx, event.DurationSec, tt.wantDur)
		}
		if tt.msgContains != "" && !contains(event.Message, tt.msgContains) {
			t.Errorf("[%d] message = %q, want substring %q", tt.idx, event.Message, tt.msgContains)
		}
	}
}

func TestParseLogTimestamp(t *testing.T) {
	events := ParseLog("2026-05-30 14:08:56 -0400 Sleep               \tEntering Sleep state 10 secs\n")
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	want := time.Date(2026, 5, 30, 14, 8, 56, 0, time.FixedZone("", -4*3600))
	if !events[0].Timestamp.Equal(want) {
		t.Errorf("timestamp = %v, want %v", events[0].Timestamp, want)
	}
}

func TestSplitDomainNoTab(t *testing.T) {
	// Some lines may use space padding rather than a tab separator.
	domain, message := splitDomain("Sleep               Entering Sleep state 5 secs")
	if domain != "Sleep" {
		t.Errorf("domain = %q, want %q", domain, "Sleep")
	}
	if message != "Entering Sleep state 5 secs" {
		t.Errorf("message = %q, want %q", message, "Entering Sleep state 5 secs")
	}
}

func TestSleepWakeEvents(t *testing.T) {
	events := ParseLog(sampleLog)
	filtered := SleepWakeEvents(events)

	// Sleep, DarkWake, Wake qualify; Assertions and "Wake Requests" do not.
	if len(filtered) != 3 {
		t.Fatalf("got %d sleep/wake events, want 3: %v", len(filtered), types(filtered))
	}
	for _, event := range filtered {
		switch event.Type {
		case "Sleep", "Wake", "DarkWake":
		default:
			t.Errorf("unexpected sleep/wake type %q", event.Type)
		}
	}
}

func TestTail(t *testing.T) {
	events := ParseLog(sampleLog)
	if got := Tail(events, 2); len(got) != 2 {
		t.Fatalf("Tail(2) len = %d, want 2", len(got))
	}
	if got := Tail(events, 100); len(got) != len(events) {
		t.Fatalf("Tail(100) len = %d, want %d", len(got), len(events))
	}
	if got := Tail(events, 0); len(got) != len(events) {
		t.Fatalf("Tail(0) len = %d, want %d", len(got), len(events))
	}
}

func TestEventString(t *testing.T) {
	events := ParseLog("2026-05-30 14:08:56 -0400 Sleep               \tEntering Sleep state 1228 secs\n")
	got := events[0].String()
	want := "2026-05-30 14:08:56  Sleep: Entering Sleep state (1228s)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func types(events []Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}
