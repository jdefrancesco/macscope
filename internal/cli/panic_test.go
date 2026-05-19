package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestRunPanicHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"panic", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(panic --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope panic --file <panic-file> [--json]") {
		t.Fatalf("stdout = %q, want panic usage", stdout.String())
	}
}

func TestRunPanicFileHuman(t *testing.T) {
	path := writePanicFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"panic", "--file", path}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(panic --file) exit code = %d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"Panic Type:",
		"WATCHDOG_TIMEOUT",
		"BOOT_IOKIT_STALL confidence=0.81",
		"external dock/display state",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunPanicFileJSON(t *testing.T) {
	path := writePanicFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"panic", "--json", "--file", path}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(panic --json --file) exit code = %d stderr=%q", code, stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if decoded["panic_type"] != "WATCHDOG_TIMEOUT" {
		t.Fatalf("panic_type = %v, want WATCHDOG_TIMEOUT", decoded["panic_type"])
	}
}

func TestParsePanicFlagsRejectsMultipleSources(t *testing.T) {
	_, err := parsePanicFlags([]string{"--last", "--file", "panic.panic"})
	if err == nil {
		t.Fatal("parsePanicFlags error = nil, want multiple-source error")
	}
}

func writePanicFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "watchdog.panic")
	data := `panic(cpu 2 caller 0xfffffe001234abcd): watchdog timeout: no checkins from watchdogd in 92 seconds
bootsessionuuid: 01234567-89AB-CDEF-0123-456789ABCDEF
osversion: 23G80,
Current Phase = "IOKit Boot"
PanicMedic Boot path active
AppleDisplayCrossbar event near panic window
Thunderbolt link reset
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
