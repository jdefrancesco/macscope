package process

import (
	"context"
	"testing"
	"time"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type fakeRunner struct {
	results map[string]collect.Result
}

func (r fakeRunner) Run(_ context.Context, name string, args ...string) (collect.Result, error) {
	key := name
	for _, arg := range args {
		key += "\x00" + arg
	}
	result, ok := r.results[key]
	if !ok {
		return collect.Result{Command: append([]string{name}, args...), ExitCode: 1}, errFake
	}
	return result, nil
}

var errFake = &collect.CommandError{
	Result: collect.Result{Command: []string{"fake"}, ExitCode: 1},
	Err:    context.DeadlineExceeded,
}

func TestParsePSLine(t *testing.T) {
	got, err := ParsePSLine("123 1 alice staff S /usr/bin/ssh /usr/bin/ssh -N example\n")
	if err != nil {
		t.Fatal(err)
	}

	if got.PID != 123 || got.PPID != 1 {
		t.Fatalf("pid/ppid = %d/%d, want 123/1", got.PID, got.PPID)
	}
	if got.Name != "ssh" {
		t.Fatalf("Name = %q, want ssh", got.Name)
	}
	if got.Path != "/usr/bin/ssh" {
		t.Fatalf("Path = %q, want /usr/bin/ssh", got.Path)
	}
}

func TestParsePID(t *testing.T) {
	if pid, ok := ParsePID("42"); !ok || pid != 42 {
		t.Fatalf("ParsePID(42) = %d/%v, want 42/true", pid, ok)
	}
	if _, ok := ParsePID("ssh"); ok {
		t.Fatal("ParsePID(ssh) ok = true, want false")
	}
}

func TestLookupByName(t *testing.T) {
	runner := fakeRunner{results: map[string]collect.Result{
		"pgrep\x00-n\x00-x\x00ssh": {
			Command:  []string{"pgrep", "-n", "-x", "ssh"},
			Stdout:   "123\n",
			ExitCode: 0,
			Duration: time.Millisecond,
		},
		"ps\x00-p\x00123\x00-o\x00pid=\x00-o\x00ppid=\x00-o\x00user=\x00-o\x00group=\x00-o\x00stat=\x00-o\x00comm=\x00-o\x00command=": {
			Command:  []string{"ps"},
			Stdout:   "123 1 alice staff S /usr/bin/ssh /usr/bin/ssh -N example\n",
			ExitCode: 0,
			Duration: time.Millisecond,
		},
	}}

	info, err := Lookup(context.Background(), "ssh", runner)
	if err != nil {
		t.Fatal(err)
	}
	if info.PID != 123 {
		t.Fatalf("PID = %d, want 123", info.PID)
	}
}
