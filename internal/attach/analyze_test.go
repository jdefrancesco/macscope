package attach

import "testing"

// TestParseGroupCheck verifies membership parsing from dseditgroup output.
func TestParseGroupCheck(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		exitOK bool
		member bool
	}{
		{
			name:   "member",
			raw:    "yes jdefr89 is a member of _developer",
			exitOK: true,
			member: true,
		},
		{
			name:   "not member",
			raw:    "no jdefr89 is NOT a member of _developer",
			exitOK: false,
			member: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseGroupCheck("_developer", tt.raw, tt.exitOK)
			if got.Member != tt.member {
				t.Fatalf("Member = %v, want %v", got.Member, tt.member)
			}
		})
	}
}

// TestAttachDenialEvidence verifies attach denial lines are extracted from logs.
func TestAttachDenialEvidence(t *testing.T) {
	got := attachDenialEvidence([]string{
		"normal log line",
		`log run noninteractively, args: 'log' 'show' '--predicate' 'eventMessage CONTAINS[c] "task_for_pid"'`,
		"taskgated task_for_pid request without decision",
		"taskgated denied task_for_pid request",
		"tccd deny Developer Tools",
	})

	if len(got) != 2 {
		t.Fatalf("evidence length = %d, want 2: %#v", len(got), got)
	}
}

// TestFilterAttachLogLines verifies only attach-relevant lines are kept.
func TestFilterAttachLogLines(t *testing.T) {
	got := filterAttachLogLines(`Timestamp                       (process)[PID]
hasAssocToWiFi6 = 1;
log run noninteractively, args: task_for_pid
audioanalyticsd NSDebugDescription failed with no attach actor
taskgated denied task_for_pid request
debugserver failed to attach`)

	if len(got) != 2 {
		t.Fatalf("filtered lines = %#v, want 2 relevant lines", got)
	}
}
