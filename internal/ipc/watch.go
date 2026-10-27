package ipc

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/process"
)

type Change struct {
	Source     string     `json:"source"`
	Kind       string     `json:"kind"`
	Name       string     `json:"name"`
	Fields     []string   `json:"fields,omitempty"`
	Before     *MachPort  `json:"before,omitempty"`
	After      *MachPort  `json:"after,omitempty"`
	FileBefore *LocalFile `json:"file_before,omitempty"`
	FileAfter  *LocalFile `json:"file_after,omitempty"`
}
type WatchEvent struct {
	CapturedAt time.Time `json:"captured_at"`
	Sequence   int       `json:"sequence"`
	Snapshot   Report    `json:"snapshot"`
	Changes    []Change  `json:"changes"`
	Warnings   []string  `json:"warnings,omitempty"`
}
type WatchOptions struct {
	Options
	Interval time.Duration
	Count    int
	// Wait is injectable so finite streams and cancellation can be tested without sleeps.
	Wait func(context.Context, time.Duration) error
}

func Watch(ctx context.Context, opts WatchOptions, emit func(WatchEvent) error) error {
	if opts.Interval < 250*time.Millisecond || opts.Interval > 24*time.Hour || opts.Count < 0 {
		return fmt.Errorf("watch interval must be between 250ms and 24h, and count must be nonnegative")
	}
	if emit == nil {
		return fmt.Errorf("watch requires an output callback")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if opts.Runner == nil {
		native, err := collect.MacOSRunner()
		if err != nil {
			return err
		}
		opts.Runner = native
	}
	info, err := process.Lookup(ctx, opts.Query, opts.Runner)
	if err != nil {
		return err
	}
	identity, err := process.Identity(ctx, info.PID, opts.Runner)
	if err != nil {
		return err
	}
	opts.Query = strconv.Itoa(identity.PID)
	wait := opts.Wait
	if wait == nil {
		wait = waitWatch
	}
	var previous Report
	for sequence := 1; opts.Count == 0 || sequence <= opts.Count; sequence++ {
		if sequence > 1 {
			if err := wait(ctx, opts.Interval); err != nil {
				return err
			}
		}
		before, err := process.Identity(ctx, identity.PID, opts.Runner)
		if err != nil {
			return fmt.Errorf("watch target exited or identity is unavailable: %w", err)
		}
		if before.StartedAt != identity.StartedAt || before.Path != identity.Path {
			return fmt.Errorf("watch stopped: PID %d changed process identity", identity.PID)
		}
		report, err := Analyze(ctx, opts.Options)
		if err != nil {
			return err
		}
		after, err := process.Identity(ctx, identity.PID, opts.Runner)
		if err != nil {
			return fmt.Errorf("watch target exited during collection: %w", err)
		}
		if after.StartedAt != identity.StartedAt || after.Path != identity.Path {
			return fmt.Errorf("watch stopped: PID %d changed identity during collection", identity.PID)
		}
		report.Process.StartedAt = identity.StartedAt
		report.Process.Path = identity.Path
		report.Process.Name = identity.Name
		event := WatchEvent{CapturedAt: time.Now().UTC(), Sequence: sequence, Snapshot: report, Changes: []Change{}}
		if sequence > 1 {
			event.Changes, event.Warnings = Compare(previous, report)
		}
		if err := emit(event); err != nil {
			return err
		}
		previous = report
	}
	return nil
}

func waitWatch(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Compare deliberately suppresses a source's deltas unless both snapshots are
// complete. Losing visibility must not look like port or descriptor removal.
func Compare(before, after Report) ([]Change, []string) {
	changes := []Change{}
	warnings := []string{}
	if before.Process.PID != after.Process.PID || before.Process.StartedAt != after.Process.StartedAt {
		return changes, []string{"different process identities; comparison skipped"}
	}
	if before.MachPorts.Status == "ok" && after.MachPorts.Status == "ok" {
		old := make(map[string]MachPort)
		newPorts := make(map[string]MachPort)
		for _, port := range before.MachPorts.Ports {
			old[port.Name] = port
		}
		for _, port := range after.MachPorts.Ports {
			newPorts[port.Name] = port
		}
		for name, port := range old {
			next, ok := newPorts[name]
			if !ok || next.IPCObject != port.IPCObject {
				copy := port
				changes = append(changes, Change{Source: "mach_ports", Kind: "removed", Name: name, Before: &copy})
			}
		}
		for name, port := range newPorts {
			previous, ok := old[name]
			copy := port
			if !ok || previous.IPCObject != port.IPCObject {
				changes = append(changes, Change{Source: "mach_ports", Kind: "added", Name: name, After: &copy})
				continue
			}
			fields := []string{}
			if !sameRights(previous.Rights, port.Rights) {
				fields = append(fields, "rights")
			}
			if !reflect.DeepEqual(previous.MessageCount, port.MessageCount) {
				fields = append(fields, "message_count")
			}
			if !reflect.DeepEqual(previous.QueueLimit, port.QueueLimit) {
				fields = append(fields, "queue_limit")
			}
			oldDetails, newDetails := previous, port
			oldDetails.Rights = nil
			newDetails.Rights = nil
			oldDetails.MessageCount = nil
			newDetails.MessageCount = nil
			oldDetails.QueueLimit = nil
			newDetails.QueueLimit = nil
			if !reflect.DeepEqual(oldDetails, newDetails) {
				fields = append(fields, "details")
			}
			if len(fields) > 0 {
				prior := previous
				changes = append(changes, Change{Source: "mach_ports", Kind: "modified", Name: name, Fields: fields, Before: &prior, After: &copy})
			}
		}
	} else {
		warnings = append(warnings, "Mach comparison skipped: both snapshots must have status ok; a new baseline is used after visibility recovers")
	}
	if before.LocalIPC.Status == "ok" && after.LocalIPC.Status == "ok" {
		old := make(map[string]LocalFile)
		newFiles := make(map[string]LocalFile)
		for _, file := range before.LocalIPC.Files {
			old[file.FD] = file
		}
		for _, file := range after.LocalIPC.Files {
			newFiles[file.FD] = file
		}
		for fd, file := range old {
			if _, ok := newFiles[fd]; !ok {
				copy := file
				changes = append(changes, Change{Source: "local_ipc", Kind: "removed", Name: fd, FileBefore: &copy})
			}
		}
		for fd, file := range newFiles {
			prior, ok := old[fd]
			copy := file
			if !ok {
				changes = append(changes, Change{Source: "local_ipc", Kind: "added", Name: fd, FileAfter: &copy})
			} else if prior != file {
				previous := prior
				changes = append(changes, Change{Source: "local_ipc", Kind: "modified", Name: fd, FileBefore: &previous, FileAfter: &copy})
			}
		}
	} else {
		warnings = append(warnings, "local IPC comparison skipped: both snapshots must have status ok")
	}
	sort.Slice(changes, func(i, j int) bool {
		a, b := changes[i], changes[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Kind < b.Kind
	})
	return changes, warnings
}

func sameRights(a, b []string) bool {
	x := append([]string{}, a...)
	y := append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	return reflect.DeepEqual(x, y)
}
