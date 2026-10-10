package ipc

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func machFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/ipc/lsmp.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseMachPorts(t *testing.T) {
	report, err := ParseMachPorts(machFixture(t), 42)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" || len(report.Ports) != 6 || *report.ReportedTotal != 6 {
		t.Fatalf("report = %#v", report)
	}
	for i, want := range [][]string{{"send"}, {"receive", "send"}, {"port_set"}, {"dead_name"}, {"send_once"}, {"send"}} {
		if !reflect.DeepEqual(report.Ports[i].Rights, want) {
			t.Errorf("port %d rights = %v, want %v", i, report.Ports[i].Rights, want)
		}
	}
	send, recv, set, unknown := report.Ports[0], report.Ports[1], report.Ports[2], report.Ports[4]
	if *send.TargetPID != 77 || send.TargetProcess != "service" || *send.MessageCount != 1 {
		t.Fatalf("send = %#v", send)
	}
	if *recv.MessageCount != 0 || *recv.QueueLimit != 5 || recv.Flags[0] != "guarded" {
		t.Fatalf("receive = %#v", recv)
	}
	if len(set.Members) != 1 || set.Members[0].Identifier != recv.Name {
		t.Fatalf("set = %#v", set)
	}
	if unknown.TargetPID != nil || unknown.TargetProcess != "" || unknown.MessageCount != nil {
		t.Fatalf("unknown destination must remain absent: %#v", unknown)
	}
}

func TestParseMachPortsBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name, data, status string
		pid                int
		wantErr            bool
	}{
		{name: "empty task", data: string(machFixture(t)), pid: 77, status: "ok"},
		{name: "missing task", data: string(machFixture(t)), pid: 999, wantErr: true},
		{name: "invalid JSON", data: "{", pid: 42, wantErr: true},
		{name: "null document", data: "null", pid: 42, wantErr: true},
		{name: "missing ports", data: `{"processes":[{"pid":42}]}`, pid: 42, wantErr: true},
		{name: "malformed port", data: `{"processes":[{"pid":42,"ports":[{}]}]}`, pid: 42, wantErr: true},
		{name: "incomplete namespace", data: `{"processes":[{"pid":42,"total":2,"ports":[]}]}`, pid: 42, status: "partial"},
		{name: "unknown total", data: `{"processes":[{"pid":42,"ports":[]}]}`, pid: 42, status: "ok"},
		{name: "negative total", data: `{"processes":[{"pid":42,"total":-1,"ports":[]}]}`, pid: 42, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMachPorts([]byte(tt.data), tt.pid)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if !tt.wantErr && (got.Status != tt.status || got.Ports == nil) {
				t.Fatalf("report = %#v", got)
			}
		})
	}
}

func TestParseLocalIPC(t *testing.T) {
	data, err := os.ReadFile("../../testdata/ipc/lsof.fields")
	if err != nil {
		t.Fatal(err)
	}
	// The fixture uses printable separators so it is reviewable in git.
	input := strings.ReplaceAll(string(data), "<NUL>", "\x00")
	files, err := ParseLocalIPC(input, 42)
	if err != nil {
		t.Fatal(err)
	}
	want := []LocalFile{
		{FD: "3", Kind: "unix_socket", Type: "unix", Access: "u", Endpoint: "/Users/alice/Library/example socket"},
		{FD: "4", Kind: "pipe", Type: "PIPE", Access: "w", Endpoint: "->0x1234"},
		{FD: "5", Kind: "fifo", Type: "FIFO", Access: "r", Endpoint: "/tmp/example fifo"},
		{FD: "6", Kind: "shared_memory", Type: "PSXSHM", Access: "u", Endpoint: "shared region"},
		{FD: "7", Kind: "semaphore", Type: "PSXSEM", Access: "u", Endpoint: "example sem"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
}

func TestParseLocalIPCBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name, input, endpoint string
		wantErr               bool
	}{
		{name: "no matching IPC", input: "p42\x00\nf1\x00tREG\x00n/tmp/file\x00\n"},
		{name: "no descriptors", input: "p42\x00\n"},
		{name: "newline in path", input: "p42\x00\nf3\x00tunix\x00n/tmp/a\nb\x00\n", endpoint: "/tmp/a\nb"},
		{name: "empty", wantErr: true},
		{name: "wrong PID", input: "p99\x00\n", wantErr: true},
		{name: "bad PID", input: "pbad\x00\n", wantErr: true},
		{name: "human table", input: "COMMAND PID USER FD TYPE NAME", wantErr: true},
		{name: "truncated field", input: "p42\x00\nf3\x00tunix\x00n/tmp/a", wantErr: true},
		{name: "missing file type", input: "p42\x00\nf3\x00n/tmp/a\x00\n", wantErr: true},
		{name: "missing process record", input: "f3\x00tunix\x00n/tmp/a\x00\n", wantErr: true},
		{name: "missing descriptor", input: "p42\x00\ntunix\x00n/tmp/a\x00\n", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLocalIPC(tt.input, 42)
			if (err != nil) != tt.wantErr {
				t.Fatalf("files = %#v, err = %v", got, err)
			}
			if !tt.wantErr {
				if got == nil || tt.endpoint == "" && len(got) != 0 || tt.endpoint != "" && (len(got) != 1 || got[0].Endpoint != tt.endpoint) {
					t.Fatalf("files = %#v", got)
				}
			}
		})
	}
}
