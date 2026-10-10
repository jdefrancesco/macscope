package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/ipc"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/process"
)

func TestIPCHelp(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"ipc", "--help"}, output.Streams{Out: &out, Err: &stderr}); code != 0 || stderr.Len() > 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{ipcUsage, "--json", "--full", "lsmp", "lsof", "task-read", "System V"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q: %s", want, out.String())
		}
	}
}

func TestParseIPCFlags(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want ipcFlags
		bad  bool
	}{
		{name: "PID", args: []string{"42"}, want: ipcFlags{query: "42"}},
		{name: "all flags", args: []string{"--json", "--full", "example"}, want: ipcFlags{json: true, full: true, query: "example"}},
		{name: "trailing flags", args: []string{"example", "--json"}, want: ipcFlags{query: "example", json: true}},
		{name: "help", args: []string{"--help"}, want: ipcFlags{help: true}},
		{name: "separator", args: []string{"--", "example"}, want: ipcFlags{query: "example"}},
		{name: "missing target", bad: true},
		{name: "empty target", args: []string{""}, bad: true},
		{name: "extra target", args: []string{"42", "43"}, bad: true},
		{name: "unknown flag", args: []string{"--all"}, bad: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIPCFlags(tt.args)
			if (err != nil) != tt.bad || got != tt.want {
				t.Fatalf("flags=%#v err=%v", got, err)
			}
		})
	}
}

func TestRenderIPCReport(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	zero, limit, pid := uint64(0), uint64(5), 77
	report := ipc.Report{
		Process: process.Info{PID: 42, Name: "example", Path: "/Users/alice/example"},
		MachPorts: ipc.MachReport{Source: "lsmp", Status: "ok", Ports: []ipc.MachPort{
			{Name: "0x103", IPCObject: "0x123", Rights: []string{"send"}, QueueLimit: &limit, MessageCount: &zero, TargetPID: &pid, TargetProcess: "service", Description: "/Users/alice/service"},
			{Name: "0x203", IPCObject: "0x456", Rights: []string{"port_set"}, Members: []ipc.MachPort{{Identifier: "0x303", Rights: []string{"receive"}}}},
		}},
		LocalIPC: ipc.LocalReport{Source: "lsof", Status: "partial", Warnings: []string{"warning: /Users/alice/example"}, Files: []ipc.LocalFile{{FD: "3", Kind: "unix_socket", Access: "u", Endpoint: "/Users/alice/test\n\x1b[31m"}}},
		Raw:      []collect.Result{{Command: []string{"lsmp"}, Stdout: "raw evidence"}},
	}
	for _, full := range []bool{false, true} {
		var out bytes.Buffer
		if err := renderIPCReport(&out, report, full); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Mach Ports:", "Local IPC:", "send", "receive", "member 0x303", "(77) service", "unix_socket", "partial", "Queue", "Queued", "TCP/UDP", "u=read/write"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("missing %q: %s", want, out.String())
			}
		}
		if strings.Contains(out.String(), "alice") != full || strings.Contains(out.String(), "raw evidence") != full {
			t.Fatalf("full=%v incorrect path/raw handling: %s", full, out.String())
		}
		if strings.ContainsRune(out.String(), '\x1b') || !strings.Contains(out.String(), `test\n\x1b[31m`) {
			t.Fatalf("unescaped endpoint controls: %q", out.String())
		}
	}
	if ipcCounter(nil) != "-" || ipcCounter(&zero) != "0" || ipcCounter(&limit) != "5" {
		t.Fatal("unknown and zero counters must be distinguishable")
	}
}

type ipcCLIRunner struct{ unavailable bool }

func (r ipcCLIRunner) Run(_ context.Context, name string, args ...string) (collect.Result, error) {
	result := collect.Result{Command: append([]string{name}, args...)}
	switch name {
	case "ps":
		result.Stdout = "42 1 alice staff S /Users/alice/example /Users/alice/example\n"
	case "/usr/bin/lsmp":
		result.Stderr = "task_for_pid() failed"
		return result, errors.New("access denied")
	case "/usr/sbin/lsof":
		if r.unavailable {
			return result, errors.New("access denied")
		}
		result.Stdout = "p42\x00\nf3\x00au\x00tunix\x00n/Users/alice/socket\x00\n"
	}
	return result, nil
}

func TestRunIPCJSONAndUnavailableSources(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		for _, full := range []bool{false, true} {
			var out bytes.Buffer
			args := []string{"--json", "42"}
			if full {
				args = append(args, "--full")
			}
			err := runIPCWithRunner(context.Background(), args, output.Streams{Out: &out}, ipcCLIRunner{unavailable: unavailable})
			if (err != nil) != unavailable {
				t.Fatalf("unavailable=%v err=%v", unavailable, err)
			}
			var report ipc.Report
			if err := json.Unmarshal(out.Bytes(), &report); err != nil {
				t.Fatalf("invalid JSON: %v: %s", err, out.String())
			}
			if report.MachPorts.Status != "unavailable" || report.MachPorts.Ports == nil || report.LocalIPC.Files == nil || report.Process.Path != "/Users/alice/example" {
				t.Fatalf("report=%#v", report)
			}
			if full && len(report.Raw) != 2 || !full && report.Raw != nil {
				t.Fatalf("full=%v raw=%#v", full, report.Raw)
			}
		}
	}
}

func TestIPCNativeJSONRendering(t *testing.T) {
	data, err := os.ReadFile("../../testdata/ipc/lsmp.json")
	if err != nil {
		t.Fatal(err)
	}
	report, err := ipc.ParseMachPorts(data, 42)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := output.WriteJSON(&out, report); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"ipc_object"`, `"message_count": 0`, `"send_once"`, `"target_pid": 77`, `"members"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("JSON missing %q: %s", want, out.String())
		}
	}
	if !json.Valid(out.Bytes()) {
		t.Fatal("invalid JSON")
	}
}
