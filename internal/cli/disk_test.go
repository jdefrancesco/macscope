package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/diskspace"
	"github.com/jdefrancesco/macscope/internal/output"
)

func TestDiskHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"disk", "--help"}, output.Streams{Out: &stdout, Err: &stderr})
	if code != 0 || stderr.Len() > 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{"macscope disk [--json] [--all] [--full] [path]", "--all", "APFS", "df -k -P -I"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q: %s", want, stdout.String())
		}
	}
}

func TestParseDiskFlags(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		want    diskFlags
		wantErr bool
	}{
		{name: "defaults"},
		{name: "all flags", args: []string{"--json", "--all", "--full", "/Volumes/Backup Drive"}, want: diskFlags{json: true, all: true, full: true, path: "/Volumes/Backup Drive"}},
		{name: "flags after path", args: []string{"/", "--json"}, want: diskFlags{json: true, path: "/"}},
		{name: "dash path", args: []string{"--", "-disk"}, want: diskFlags{path: "-disk"}},
		{name: "help", args: []string{"--help"}, want: diskFlags{help: true}},
		{name: "unknown", args: []string{"--bad"}, wantErr: true},
		{name: "extra paths", args: []string{"/", "/tmp"}, wantErr: true},
		{name: "empty path", args: []string{""}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDiskFlags(tt.args)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("flags = %#v, error = %v, want %#v", got, err, tt.want)
			}
		})
	}
}

func TestRenderDiskReport(t *testing.T) {
	report := diskspace.Report{
		RequestedPath: "/Users/alice/projects",
		Volumes: []diskspace.Volume{{
			Filesystem: "devices -- file:///Users/alice/Library/", MountPoint: "/Users/alice/Disk",
			TotalBytes: 1024 * 1024 * 1024, UsedBytes: 256 * 1024 * 1024, AvailableBytes: 512 * 1024 * 1024, CapacityPercent: 34,
		}},
	}
	for _, full := range []bool{false, true} {
		var buf bytes.Buffer
		if err := renderDiskReport(&buf, report, full); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Disk Space:", "Filesystem", "Size", "Used", "Free", "Use%", "1.0 GiB", "256.0 MiB", "512.0 MiB", "34%", "APFS"} {
			if !strings.Contains(buf.String(), want) {
				t.Fatalf("output missing %q: %s", want, buf.String())
			}
		}
		if strings.Contains(buf.String(), "alice") != full {
			t.Fatalf("full=%v, incorrect username redaction: %s", full, buf.String())
		}
		if !full && !strings.Contains(buf.String(), "/Users/<redacted>") {
			t.Fatalf("redacted path missing: %s", buf.String())
		}
	}
	var buf bytes.Buffer
	if err := output.WriteJSON(&buf, report); err != nil {
		t.Fatal(err)
	}
	var decoded diskspace.Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.RequestedPath != report.RequestedPath || decoded.Volumes[0] != report.Volumes[0] {
		t.Fatalf("JSON lost raw paths or exact byte counts: %s", buf.String())
	}
}

func TestFormatBytes(t *testing.T) {
	for _, tt := range []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"}, {1023, "1023 B"}, {1024, "1.0 KiB"}, {1024 * 1024, "1.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"}, {-1024, "-1.0 KiB"},
	} {
		if got := formatBytes(tt.bytes); got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
