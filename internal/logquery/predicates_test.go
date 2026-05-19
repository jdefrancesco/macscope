package logquery

import (
	"strings"
	"testing"
)

func TestPredicate(t *testing.T) {
	tests := []struct {
		name string
		want string
		ok   bool
	}{
		{name: "tcc", want: "tccd", ok: true},
		{name: "attach", want: "task_for_pid", ok: true},
		{name: "es", want: "EndpointSecurity", ok: true},
		{name: "vpn", want: "utun", ok: true},
		{name: "panic", want: "watchdog", ok: true},
		{name: "missing", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Predicate(tt.name)
			if ok != tt.ok {
				t.Fatalf("Predicate(%q) ok = %v, want %v", tt.name, ok, tt.ok)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Fatalf("Predicate(%q) = %q, want substring %q", tt.name, got, tt.want)
			}
		})
	}
}
