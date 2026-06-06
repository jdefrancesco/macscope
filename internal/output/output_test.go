package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer

	err := WriteJSON(&buf, map[string]any{
		"panic_type":           "WATCHDOG_TIMEOUT",
		"watchdog_timeout_sec": 92,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(buf.Bytes()) {
		t.Fatalf("WriteJSON produced invalid JSON: %q", buf.String())
	}
	if !strings.Contains(buf.String(), `"panic_type": "WATCHDOG_TIMEOUT"`) {
		t.Fatalf("WriteJSON output = %q, want panic_type", buf.String())
	}
}

func TestTextWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := NewTextWriter(&buf)

	if err := writer.Section("Panic Type"); err != nil {
		t.Fatal(err)
	}
	if err := writer.KeyValue("Summary", "watchdogd missed checkins"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Bullet("panic string contains watchdog timeout"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Detail("source: panic-full-1234.panic"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Section("Metadata"); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	for _, want := range []string{
		"Panic Type:",
		"Summary:",
		"- panic string contains watchdog timeout",
		"- source: panic-full-1234.panic",
		"\n\nMetadata:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("TextWriter output missing %q:\n%s", want, got)
		}
	}
}

func TestNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	if got := Level("HIGH"); got != "HIGH" {
		t.Fatalf("Level with NO_COLOR = %q, want HIGH", got)
	}
}
