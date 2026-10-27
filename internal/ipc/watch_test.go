package ipc

import (
	"context"
	"errors"
	"github.com/jdefrancesco/macscope/internal/collect"
	"testing"
	"time"

	"github.com/jdefrancesco/macscope/internal/process"
)

func TestCompareSnapshots(t *testing.T) {
	q1, q2 := uint64(1), uint64(4)
	before := Report{Process: process.Info{PID: 42, StartedAt: "same"}, MachPorts: MachReport{Status: "ok", Ports: []MachPort{{Name: "0x103", IPCObject: "0x1", Rights: []string{"send"}, MessageCount: &q1}, {Name: "0x203", IPCObject: "0x2"}}}, LocalIPC: LocalReport{Status: "ok", Files: []LocalFile{{FD: "3", Kind: "pipe", Endpoint: "old"}}}}
	after := Report{Process: before.Process, MachPorts: MachReport{Status: "ok", Ports: []MachPort{{Name: "0x103", IPCObject: "0x1", Rights: []string{"send", "receive"}, MessageCount: &q2}, {Name: "0x303", IPCObject: "0x3"}}}, LocalIPC: LocalReport{Status: "ok", Files: []LocalFile{{FD: "3", Kind: "pipe", Endpoint: "new"}}}}
	changes, warnings := Compare(before, after)
	if len(changes) != 4 || len(warnings) != 0 {
		t.Fatalf("changes=%#v warnings=%v", changes, warnings)
	}
	var modified bool
	for _, change := range changes {
		if change.Name == "0x103" {
			modified = change.Kind == "modified" && len(change.Fields) == 2 && *change.After.MessageCount == 4
		}
	}
	if !modified {
		t.Fatalf("missing rights/queue change: %#v", changes)
	}
	for _, status := range []string{"partial", "unavailable"} {
		after.MachPorts.Status = status
		after.MachPorts.Ports = nil
		changes, warnings = Compare(before, after)
		if len(changes) != 1 || len(warnings) == 0 || changes[0].Source != "local_ipc" {
			t.Fatalf("false port removals: %#v %v", changes, warnings)
		}
	}
	after.Process.StartedAt = "different"
	changes, warnings = Compare(before, after)
	if len(changes) != 0 || len(warnings) == 0 {
		t.Fatal("compared reused PID")
	}
}

type watchRunner struct {
	base       fakeRunner
	identities int
	changeAt   int
	exitAt     int
}

func (r *watchRunner) Run(ctx context.Context, name string, args ...string) (collect.Result, error) {
	if name != "/bin/ps" {
		return r.base.Run(ctx, name, args...)
	}
	r.identities++
	if r.exitAt > 0 && r.identities >= r.exitAt {
		return collect.Result{}, errors.New("process exited")
	}
	start := "09:00:00"
	if r.changeAt > 0 && r.identities >= r.changeAt {
		start = "09:00:01"
	}
	return collect.Result{Stdout: "42 1 Mon Oct 26 " + start + " 2026 /Users/alice/example\n"}, nil
}

func TestWatchFiniteAndTargetLifecycle(t *testing.T) {
	for _, tt := range []struct {
		name                      string
		change, exit, count, want int
		bad                       bool
	}{{"finite", 0, 0, 2, 2, false}, {"PID reuse", 4, 0, 2, 1, true}, {"exit before next sample", 0, 4, 2, 1, true}, {"exit during sample", 0, 3, 1, 0, true}} {
		t.Run(tt.name, func(t *testing.T) {
			r := &watchRunner{base: fakeRunner{mach: machFixture(t), lsof: "p42\x00\nf3\x00tPIPE\x00n->0x123\x00\n"}, changeAt: tt.change, exitAt: tt.exit}
			var events []WatchEvent
			waits := 0
			err := Watch(context.Background(), WatchOptions{Options: Options{Query: "example", Runner: r}, Interval: time.Second, Count: tt.count, Wait: func(ctx context.Context, _ time.Duration) error { waits++; return ctx.Err() }}, func(event WatchEvent) error { events = append(events, event); return nil })
			if (err != nil) != tt.bad || len(events) != tt.want {
				t.Fatalf("events=%d err=%v", len(events), err)
			}
			if !tt.bad && (waits != 1 || events[1].Sequence != 2 || events[1].CapturedAt.IsZero() || events[1].Snapshot.Process.StartedAt == "") {
				t.Fatal("invalid finite stream")
			}
		})
	}
	r := &watchRunner{base: fakeRunner{mach: machFixture(t), lsof: "p42\x00\n"}}
	sentinel := errors.New("output failed")
	err := Watch(context.Background(), WatchOptions{Options: Options{Query: "42", Runner: r}, Interval: time.Second, Count: 1}, func(WatchEvent) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("output error=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r = &watchRunner{base: fakeRunner{mach: machFixture(t), lsof: "p42\x00\n"}}
	err = Watch(ctx, WatchOptions{Options: Options{Query: "42", Runner: r}, Interval: time.Hour}, func(WatchEvent) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
}

func TestWatchValidationAndCancellation(t *testing.T) {
	for _, opts := range []WatchOptions{{Interval: time.Millisecond}, {Interval: time.Second, Count: -1}} {
		if err := Watch(context.Background(), opts, func(WatchEvent) error { return nil }); err == nil {
			t.Fatal("invalid watch accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitWatch(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
}
