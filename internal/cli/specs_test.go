package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/systeminfo"
)

func TestSpecsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"specs", "--help"}, output.Streams{Out: &stdout, Err: &stderr})
	if code != 0 || stderr.Len() > 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{"macscope specs [--json]", "system_profiler", "sysctl", "sw_vers", "requires macOS"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q: %s", want, stdout.String())
		}
	}
}

func TestParseSpecsFlags(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		json    bool
		help    bool
		wantErr bool
	}{
		{name: "defaults"},
		{name: "json", args: []string{"--json"}, json: true},
		{name: "help", args: []string{"-h"}, help: true},
		{name: "unknown", args: []string{"--bad"}, wantErr: true},
		{name: "unexpected path", args: []string{"/"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSpecsFlags(tt.args)
			if (err != nil) != tt.wantErr || got.json != tt.json || got.help != tt.help {
				t.Fatalf("flags = %#v, error = %v", got, err)
			}
		})
	}
}

func TestRenderSpecsReport(t *testing.T) {
	report := systeminfo.Report{
		Hardware: systeminfo.Hardware{
			ModelName: "MacBook Pro", ModelIdentifier: "Mac16,8", Processor: "Apple M4 Pro",
			MemoryBytes: 48 * 1024 * 1024 * 1024, PhysicalCores: 14, LogicalCores: 14, Architecture: "arm64",
			ExecutionArchitecture: "x86_64",
			Graphics:              []systeminfo.Graphics{{Model: "Apple M4 Pro", Cores: 20}, {Model: "External GPU", VRAM: "8 GB"}},
		},
		OS: systeminfo.OS{Name: "macOS", Version: "15.5", Build: "24F74"},
	}
	var buf bytes.Buffer
	if err := renderSpecsReport(&buf, report); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"System Specs:", "MacBook Pro (Mac16,8)", "14 physical / 14 logical", "48.0 GiB", "macOS 15.5 (build 24F74)", "20 cores", "VRAM 8 GB", "Execution Architecture", "x86_64"} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q: %s", want, buf.String())
		}
	}
}
