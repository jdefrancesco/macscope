package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/evidence"
	"github.com/jdefrancesco/macscope/internal/memory"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/process"
	"github.com/jdefrancesco/macscope/internal/proctriage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeFlags(t *testing.T) {
	proc, err := parseProcFlags([]string{"--maps", "--tree", "--full", "--json", "42"})
	if err != nil || !proc.maps || !proc.tree || !proc.full || !proc.json {
		t.Fatalf("flags=%#v err=%v", proc, err)
	}
	watch, err := parseIPCFlags([]string{"--watch", "--jsonl", "--interval", "500ms", "--count", "2", "42"})
	if err != nil || !watch.jsonl || watch.interval != 500*time.Millisecond || watch.count != 2 {
		t.Fatalf("flags=%#v err=%v", watch, err)
	}
	for _, args := range [][]string{{"--jsonl", "42"}, {"--watch", "--json", "42"}, {"--interval", "2s", "42"}, {"--watch", "--interval", "0", "42"}, {"--watch", "--count", "0", "42"}, {"--watch", "--count"}, {"--watch", "--interval"}} {
		if _, err := parseIPCFlags(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, args := range [][]string{{}, {"--pid", "0", "--out", "test"}, {"--pid", "42"}, {"--pid"}, {"--pid", "42", "--out", "test", "--reports", "maps,maps"}, {"--pid", "42", "--out", "test", "--reports", "bad"}, {"--bad"}} {
		if _, err := parseCollectFlags(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	flags, err := parseCollectFlags([]string{"--pid", "42", "--out", "test", "--reports", "maps, tree", "--json"})
	if err != nil || flags.pid != 42 || len(flags.reports) != 2 || !flags.json {
		t.Fatalf("flags=%#v err=%v", flags, err)
	}
}

type runtimeCLIRunner struct{ ipcCLIRunner }

func (r runtimeCLIRunner) Run(ctx context.Context, name string, args ...string) (collect.Result, error) {
	switch name {
	case "/bin/ps":
		if args[0] == "-axww" {
			data, err := os.ReadFile("../../testdata/process/tree.txt")
			return collect.Result{Stdout: string(data)}, err
		}
		return collect.Result{Stdout: "42 20 Mon Oct 26 09:00:00 2026 /Applications/Example App.app/Contents/MacOS/Example App\n"}, nil
	case "/usr/bin/vmmap":
		data, err := os.ReadFile("../../testdata/memory/vmmap.txt")
		return collect.Result{Stdout: string(data)}, err
	default:
		return r.ipcCLIRunner.Run(ctx, name, args...)
	}
}

func TestCollectJSONAndWatchJSONL(t *testing.T) {
	var out, stderr bytes.Buffer
	dir := filepath.Join(t.TempDir(), "bundle")
	args := []string{"--pid", "42", "--out", dir, "--reports", "maps,tree", "--json"}
	if err := runCollectWithRunner(context.Background(), args, output.Streams{Out: &out, Err: &stderr}, runtimeCLIRunner{}); err != nil {
		t.Fatal(err)
	}
	var manifest evidence.Manifest
	if err := json.Unmarshal(out.Bytes(), &manifest); err != nil || len(manifest.Reports) != 2 || manifest.ManifestPath != "manifest.json" || len(manifest.Files) != 3 {
		t.Fatalf("manifest=%#v err=%v output=%s", manifest, err, out.String())
	}
	if !strings.Contains(stderr.String(), "raw evidence locally") {
		t.Fatal("missing privacy notice")
	}
	out.Reset()
	if err := runCollectWithRunner(context.Background(), args, output.Streams{Out: &out, Err: &stderr}, runtimeCLIRunner{}); err == nil || out.Len() != 0 {
		t.Fatal("existing directory was not refused")
	}
	out.Reset()
	if err := runIPCWithRunner(context.Background(), []string{"--watch", "--count", "1", "--jsonl", "42"}, output.Streams{Out: &out, Err: &stderr}, runtimeCLIRunner{}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 || !json.Valid([]byte(lines[0])) {
		t.Fatalf("invalid JSONL: %s", out.String())
	}
}

func TestCollectHelpAndRenderers(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"collect", "--help"}, output.Streams{Out: &out, Err: &stderr}); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d err=%s", code, stderr.String())
	}
	for _, word := range []string{collectUsage, "0700", "0600", "SHA-256", "raw", "network"} {
		if !strings.Contains(out.String(), word) {
			t.Errorf("help missing %s", word)
		}
	}
	var buf bytes.Buffer
	report := proctriage.Report{Process: process.Info{PID: 42, Path: "/Users/alice/app"}, Maps: &memory.Report{Status: "ok", Images: []memory.Image{{Path: "/Users/alice/lib.dylib"}}, Regions: []memory.Region{{Start: "0x1000", End: "0x2000", SizeBytes: 4096, Protection: "r-x", MaxProtection: "rwx", Path: "/Users/alice/lib.dylib"}}}, Tree: &process.Tree{Status: "ok", Target: process.Node{PID: 42, Path: "/Users/alice/app"}, Children: []process.Node{{PID: 77, Path: "/Users/alice/helper"}}}}
	if err := renderProcReportFull(&buf, report, false); err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"Runtime Memory Maps", "Process Ancestry", "r-x/rwx", "4.0 KiB", "Child pid=77"} {
		if !strings.Contains(buf.String(), word) {
			t.Errorf("missing %s: %s", word, buf.String())
		}
	}
	if strings.Contains(buf.String(), "alice") {
		t.Fatal("path leaked")
	}
	buf.Reset()
	if err := renderBundle(&buf, evidence.Manifest{Directory: "/Users/alice/evidence", Status: "partial"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "not written") || strings.Contains(buf.String(), "alice") {
		t.Fatal(buf.String())
	}
	data, err := json.Marshal(report)
	if err != nil || !json.Valid(data) || !strings.Contains(string(data), "/Users/alice/lib.dylib") {
		t.Fatalf("json=%s err=%v", data, err)
	}
}
