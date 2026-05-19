package timeline

import (
	"strings"
	"testing"
)

func TestTimelinePredicateIncludesPID(t *testing.T) {
	got := TimelinePredicate(123)
	for _, want := range []string{`pid 123`, `[123]`, `task_for_pid`} {
		if !strings.Contains(got, want) {
			t.Fatalf("TimelinePredicate missing %q: %s", want, got)
		}
	}
}

func TestParseLogEvents(t *testing.T) {
	input := `Timestamp                       (process)[PID]
2026-05-19 15:11:00.000000-0400 localhost taskgated[99]: denied task_for_pid request for pid 123
2026-05-19 15:11:01.000000-0400 localhost tccd[88]: deny kTCCServiceDeveloperTool client=com.example.Terminal
2026-05-19 15:11:02.000000-0400 localhost other[77]: normal log line`

	events := ParseLogEvents(input)
	if len(events) != 2 {
		t.Fatalf("events length = %d, want 2: %#v", len(events), events)
	}
	if events[0].Category != "ATTACH_POLICY_DENIAL" {
		t.Fatalf("first category = %q", events[0].Category)
	}
	if events[1].Category != "TCC_DENIAL" {
		t.Fatalf("second category = %q", events[1].Category)
	}
}

func TestLogFindings(t *testing.T) {
	events := []Event{
		{Category: "ATTACH_POLICY_DENIAL", Severity: "medium", Evidence: "denied task_for_pid"},
		{Category: "LOG_EVENT", Severity: "info", Evidence: "normal"},
	}

	findings := logFindings(events)
	if len(findings) != 1 || findings[0].Category != "ATTACH_POLICY_DENIAL" {
		t.Fatalf("findings = %#v", findings)
	}
}
