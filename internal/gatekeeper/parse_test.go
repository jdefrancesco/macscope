package gatekeeper

import "testing"

func TestParseAssessment(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		exitCode int
		accepted bool
		source   string
	}{
		{
			name:     "accepted",
			output:   "/bin/ls: accepted\nsource=Apple System\norigin=Software Signing",
			exitCode: 0,
			accepted: true,
			source:   "Apple System",
		},
		{
			name:     "rejected",
			output:   "/tmp/tool: rejected\nsource=no usable signature",
			exitCode: 3,
			accepted: false,
			source:   "no usable signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAssessment(tt.output, "", tt.exitCode)
			if got.Accepted != tt.accepted {
				t.Fatalf("Accepted = %v, want %v", got.Accepted, tt.accepted)
			}
			if got.Source != tt.source {
				t.Fatalf("Source = %q, want %q", got.Source, tt.source)
			}
		})
	}
}
