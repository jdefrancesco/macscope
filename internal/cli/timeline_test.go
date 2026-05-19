package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/process"
	"github.com/jdefrancesco/macscope/internal/timeline"
)

func TestRunTimelineHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"timeline", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(timeline --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope timeline --pid <pid> [--last 30m] [--json|--jsonl]") {
		t.Fatalf("stdout = %q, want timeline usage", stdout.String())
	}
}

func TestParseTimelineFlags(t *testing.T) {
	flags, err := parseTimelineFlags([]string{"--pid", "123", "--last", "10m", "--jsonl"})
	if err != nil {
		t.Fatal(err)
	}
	if flags.pid != 123 || flags.last != "10m" || !flags.jsonl {
		t.Fatalf("flags = %#v, want pid/last/jsonl", flags)
	}
}

func TestRenderTimelineReport(t *testing.T) {
	var buf bytes.Buffer
	report := timeline.Report{
		PID:    123,
		Window: "30m",
		Process: process.Info{
			PID:     123,
			Name:    "demo",
			User:    "alice",
			Path:    "/tmp/demo",
			Command: "/tmp/demo",
		},
		Events: []timeline.Event{
			{Source: "ps", Category: "PROCESS_OBSERVED", Severity: "info", Message: "pid=123"},
			{Source: "taskgated", Category: "ATTACH_POLICY_DENIAL", Severity: "medium", Message: "denied task_for_pid"},
		},
		Findings: []timeline.Finding{
			{Category: "ATTACH_POLICY_DENIAL", Severity: "medium", Confidence: 0.78, Evidence: []string{"denied task_for_pid"}, Source: "log show timeline predicate"},
		},
	}

	if err := renderTimelineReport(&buf, report); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Timeline:", "ATTACH_POLICY_DENIAL", "denied task_for_pid"} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestWriteTimelineJSONL(t *testing.T) {
	var buf bytes.Buffer
	report := timeline.Report{
		Events: []timeline.Event{
			{Source: "ps", Category: "PROCESS_OBSERVED", Severity: "info", Message: "pid=123"},
		},
	}

	if err := writeTimelineJSONL(&buf, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"category":"PROCESS_OBSERVED"`) {
		t.Fatalf("jsonl = %q", buf.String())
	}
}
