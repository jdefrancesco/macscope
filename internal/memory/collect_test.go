package memory

import (
	"context"
	"errors"
	"github.com/jdefrancesco/macscope/internal/collect"
	"os"
	"reflect"
	"testing"
)

type mapRunner struct {
	stdout, stderr string
	err            error
	call           []string
}

func (r *mapRunner) Run(_ context.Context, name string, args ...string) (collect.Result, error) {
	r.call = append([]string{name}, args...)
	return collect.Result{Command: r.call, Stdout: r.stdout, Stderr: r.stderr}, r.err
}

func TestCollect(t *testing.T) {
	fixture, err := os.ReadFile("../../testdata/memory/vmmap.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, status string
		runner       mapRunner
		full         bool
	}{{"ok", "ok", mapRunner{stdout: string(fixture)}, false}, {"raw", "ok", mapRunner{stdout: string(fixture)}, true}, {"denied", "unavailable", mapRunner{stderr: "permission denied", err: errors.New("exit 1")}, false}, {"partial", "partial", mapRunner{stdout: string(fixture), stderr: "some data unavailable", err: errors.New("exit 1")}, false}} {
		t.Run(tt.name, func(t *testing.T) {
			report, err := Collect(context.Background(), 42, tt.full, &tt.runner)
			if err != nil || report.Status != tt.status || report.CapturedAt.IsZero() {
				t.Fatalf("report=%#v err=%v", report, err)
			}
			if (report.Raw != nil) != tt.full {
				t.Fatal("raw policy mismatch")
			}
			if !reflect.DeepEqual(tt.runner.call, []string{"/usr/bin/vmmap", "-w", "42"}) {
				t.Fatal(tt.runner.call)
			}
			if tt.status != "ok" && len(report.Warnings) == 0 {
				t.Fatal("missing warning")
			}
		})
	}
	if _, err := Collect(context.Background(), 0, false, &mapRunner{}); err == nil {
		t.Fatal("accepted zero PID")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Collect(ctx, 42, false, &mapRunner{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}
