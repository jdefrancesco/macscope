// Package evidence writes private, local diagnostic bundles without overwriting
// existing evidence or submitting anything over the network.
package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/ipc"
	"github.com/jdefrancesco/macscope/internal/memory"
	"github.com/jdefrancesco/macscope/internal/process"
	"github.com/jdefrancesco/macscope/internal/proctriage"
)

type Options struct {
	PID     int
	Out     string
	Reports []string
	Version string
	Commit  string
	Runner  process.CommandRunner
}
type File struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}
type ReportEntry struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Error       string    `json:"error,omitempty"`
}
type Manifest struct {
	SchemaVersion int           `json:"schema_version"`
	Version       string        `json:"macscope_version"`
	Commit        string        `json:"macscope_commit"`
	Directory     string        `json:"directory"`
	ManifestPath  string        `json:"manifest_path,omitempty"`
	PID           int           `json:"pid"`
	StartedAt     string        `json:"process_started_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	CompletedAt   time.Time     `json:"completed_at"`
	Status        string        `json:"status"`
	Reports       []ReportEntry `json:"reports"`
	Files         []File        `json:"files"`
	Warnings      []string      `json:"warnings,omitempty"`
}
type CommandRecord struct {
	CapturedAt  time.Time      `json:"captured_at"`
	CompletedAt time.Time      `json:"completed_at"`
	Result      collect.Result `json:"result"`
	Error       string         `json:"error,omitempty"`
}
type recordingRunner struct {
	runner  process.CommandRunner
	records []CommandRecord
}

func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) (collect.Result, error) {
	started := time.Now().UTC()
	result, err := r.runner.Run(ctx, name, args...)
	result.Command = append([]string{name}, args...)
	record := CommandRecord{CapturedAt: started, CompletedAt: time.Now().UTC(), Result: result}
	if err != nil {
		record.Error = err.Error()
	}
	r.records = append(r.records, record)
	return result, err
}

func ValidateReports(reports []string) error {
	seen := map[string]bool{}
	for _, name := range reports {
		if name != "proc" && name != "ipc" && name != "maps" && name != "tree" {
			return fmt.Errorf("unknown report %q; choose proc,ipc,maps,tree", name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate report %q", name)
		}
		seen[name] = true
	}
	return nil
}

func Create(ctx context.Context, opts Options) (Manifest, error) {
	if opts.PID <= 0 || opts.Out == "" {
		return Manifest{}, fmt.Errorf("bundle requires a positive PID and an output directory")
	}
	if len(opts.Reports) == 0 {
		opts.Reports = []string{"proc", "ipc", "maps", "tree"}
	}
	if err := ValidateReports(opts.Reports); err != nil {
		return Manifest{}, err
	}
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	runner := opts.Runner
	if runner == nil {
		native, err := collect.MacOSRunner()
		if err != nil {
			return Manifest{}, err
		}
		runner = native
	}
	dir, err := filepath.Abs(opts.Out)
	if err != nil {
		return Manifest{}, err
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return Manifest{}, fmt.Errorf("create private evidence directory (must not already exist): %w", err)
	}
	manifest := Manifest{SchemaVersion: 1, Version: opts.Version, Commit: opts.Commit, Directory: dir, PID: opts.PID, CreatedAt: time.Now().UTC(), Status: "ok", Reports: []ReportEntry{}, Files: []File{}}
	recorder := &recordingRunner{runner: runner, records: []CommandRecord{}}
	identity, identityErr := process.Identity(ctx, opts.PID, recorder)
	if identityErr != nil {
		manifest.Warnings = append(manifest.Warnings, identityErr.Error())
		manifest.Status = "partial"
	} else {
		manifest.StartedAt = identity.StartedAt
	}
	for _, name := range opts.Reports {
		if ctx.Err() != nil {
			manifest.Warnings = append(manifest.Warnings, "collection canceled; only completed reports are present")
			manifest.Status = "partial"
			break
		}
		entry := ReportEntry{Name: name, StartedAt: time.Now().UTC(), Status: "ok"}
		var value any
		reportErr := identityErr
		if identityErr == nil {
			current, err := process.Identity(ctx, opts.PID, recorder)
			if err != nil {
				reportErr = err
			} else if current.StartedAt != identity.StartedAt || current.Path != identity.Path {
				reportErr = fmt.Errorf("target identity changed; refusing to collect a different process")
			}
		}
		if reportErr == nil {
			value, entry.Status, reportErr = collectReport(ctx, name, opts.PID, recorder)
		}
		if reportErr != nil {
			entry.Status = "unavailable"
			entry.Error = reportErr.Error()
			value = map[string]string{"status": "unavailable", "error": entry.Error}
		}
		entry.CompletedAt = time.Now().UTC()
		file, err := writeJSON(dir, name+".json", value)
		if err != nil {
			manifest.Status = "partial"
			manifest.Warnings = append(manifest.Warnings, "write report: "+err.Error())
			return manifest, fmt.Errorf("preserved partial evidence in %s: %w", dir, err)
		}
		manifest.Files = append(manifest.Files, file)
		manifest.Reports = append(manifest.Reports, entry)
		if entry.Status != "ok" {
			manifest.Status = "partial"
		}
	}
	if identityErr == nil {
		after, err := process.Identity(ctx, opts.PID, recorder)
		if err != nil || after.StartedAt != identity.StartedAt || after.Path != identity.Path {
			manifest.Warnings = append(manifest.Warnings, "target exited, changed, or became inaccessible during collection; snapshots are not atomic")
			manifest.Status = "partial"
		}
	}
	raw, err := writeJSON(dir, "commands.json", recorder.records)
	if err != nil {
		manifest.Status = "partial"
		manifest.Warnings = append(manifest.Warnings, "write commands: "+err.Error())
		return manifest, fmt.Errorf("preserved partial evidence in %s: %w", dir, err)
	}
	manifest.Files = append(manifest.Files, raw)
	manifest.CompletedAt = time.Now().UTC()
	manifest.ManifestPath = "manifest.json"
	if _, err := writeJSON(dir, "manifest.json", manifest); err != nil {
		manifest.Status = "partial"
		manifest.ManifestPath = ""
		manifest.Warnings = append(manifest.Warnings, "write manifest: "+err.Error())
		return manifest, fmt.Errorf("preserved evidence in %s; manifest: %w", dir, err)
	}
	return manifest, ctx.Err()
}

func collectReport(ctx context.Context, name string, pid int, runner process.CommandRunner) (any, string, error) {
	switch name {
	case "proc":
		report, err := proctriage.AnalyzeWithOptions(ctx, proctriage.Options{Query: strconv.Itoa(pid), Runner: runner})
		status := "ok"
		if len(report.Warnings) > 0 {
			status = "partial"
		}
		return report, status, err
	case "ipc":
		report, err := ipc.Analyze(ctx, ipc.Options{Query: strconv.Itoa(pid), Runner: runner})
		status := "ok"
		if report.MachPorts.Status != "ok" || report.LocalIPC.Status != "ok" {
			status = "partial"
		}
		if report.MachPorts.Status == "unavailable" && report.LocalIPC.Status == "unavailable" {
			status = "unavailable"
		}
		return report, status, err
	case "maps":
		report, err := memory.Collect(ctx, pid, false, runner)
		return report, report.Status, err
	case "tree":
		report, err := process.SnapshotTree(ctx, pid, runner)
		return report, report.Status, err
	default:
		return nil, "unavailable", fmt.Errorf("unknown report: %s", name)
	}
}

func writeJSON(dir, name string, value any) (File, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return File{}, err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return File{}, err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return File{}, writeErr
	}
	if closeErr != nil {
		return File{}, closeErr
	}
	sum := sha256.Sum256(data)
	return File{Path: name, SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}, nil
}
