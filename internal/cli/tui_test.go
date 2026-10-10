package cli

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestTUIEntriesOrderAndExclusions(t *testing.T) {
	entries := tuiEntries()

	var got []string
	for _, e := range entries {
		got = append(got, e.Name)
	}

	presets := tuiPresets()
	var want []string
	for _, cmd := range commands() {
		if _, ok := presets[cmd.Name]; ok {
			want = append(want, cmd.Name)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tuiEntries names = %v, want %v", got, want)
	}
	if len(got) != len(presets) {
		t.Fatalf("tuiEntries len = %d, want %d (every preset should match a command)", len(got), len(presets))
	}
	for _, excluded := range []string{"completion", "tui"} {
		for _, name := range got {
			if name == excluded {
				t.Fatalf("tuiEntries includes %q", excluded)
			}
		}
	}
}

func TestTUIEntriesPresets(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		prompt string
	}{
		{name: "version"},
		{name: "specs"},
		{name: "disk"},
		{name: "persist"},
		{name: "agents"},
		{name: "daemons"},
		{name: "sysext"},
		{name: "vpn"},
		{name: "tcc", args: []string{"--last", "30m"}},
		{name: "es", args: []string{"--last", "30m"}},
		{name: "panic", args: []string{"--last"}},
		{name: "macho", prompt: "path"},
		{name: "proc", prompt: "pid or name"},
		{name: "ipc", prompt: "pid or name"},
		{name: "attach", prompt: "pid"},
		{name: "timeline", args: []string{"--pid"}, prompt: "pid"},
	}

	byName := map[string]int{}
	entries := tuiEntries()
	for i, e := range entries {
		byName[e.Name] = i
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, ok := byName[tt.name]
			if !ok {
				t.Fatalf("entry %q missing", tt.name)
			}
			e := entries[idx]
			if len(e.Args) != len(tt.args) || (len(tt.args) > 0 && !reflect.DeepEqual(e.Args, tt.args)) {
				t.Fatalf("Args = %v, want %v", e.Args, tt.args)
			}
			if e.Prompt != tt.prompt {
				t.Fatalf("Prompt = %q, want %q", e.Prompt, tt.prompt)
			}
			if e.Run == nil {
				t.Fatal("Run is nil")
			}
			if e.Summary == "" {
				t.Fatal("Summary is empty")
			}
		})
	}
}

func TestTUIEntriesArgsAreIndependent(t *testing.T) {
	first := tuiEntries()
	for i := range first {
		if len(first[i].Args) > 0 {
			first[i].Args[0] = "mutated"
		}
	}
	for _, e := range tuiEntries() {
		for _, arg := range e.Args {
			if arg == "mutated" {
				t.Fatalf("entry %q shares Args backing array across calls", e.Name)
			}
		}
	}
}

func TestRunTUI(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    string
		wantOut    []string
		wantTermEr bool
	}{
		{name: "non-terminal streams", wantTermEr: true},
		{name: "help long", args: []string{"--help"}, wantOut: []string{"macscope tui", "enter run", "--json"}},
		{name: "help short", args: []string{"-h"}, wantOut: []string{"macscope tui"}},
		{name: "unknown arg", args: []string{"--bogus"}, wantErr: "unknown tui argument: --bogus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runTUI(context.Background(), tt.args, output.Streams{
				In:  &bytes.Buffer{},
				Out: &stdout,
				Err: &stderr,
			})
			switch {
			case tt.wantTermEr:
				if !errors.Is(err, errTUINotTerminal) {
					t.Fatalf("err = %v, want %v", err, errTUINotTerminal)
				}
			case tt.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
			default:
				if err != nil {
					t.Fatalf("err = %v", err)
				}
			}
			for _, want := range tt.wantOut {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
				}
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		name string
		v    any
	}{
		{name: "buffer", v: &bytes.Buffer{}},
		{name: "nil", v: nil},
		{name: "string", v: "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isTerminal(tt.v) {
				t.Fatalf("isTerminal(%T) = true, want false", tt.v)
			}
		})
	}
}
