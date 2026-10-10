package ipc

import (
	"context"
	"errors"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type fakeRunner struct {
	mach       []byte
	machErr    error
	machStderr string
	lsof       string
	lsofErr    error
	lsofStderr string
	lookupErr  error
	calls      [][]string
	outputPath string
	outputMode os.FileMode
	cancel     context.CancelFunc
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) (collect.Result, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	result := collect.Result{Command: call}
	switch name {
	case "pgrep":
		result.Stdout = "42\n"
	case "ps":
		result.Stdout = "42 1 alice staff S /Users/alice/example /Users/alice/example --option\n"
		return result, r.lookupErr
	case "/usr/bin/lsmp":
		r.outputPath = args[3]
		info, err := os.Stat(r.outputPath)
		if err != nil {
			return result, err
		}
		r.outputMode = info.Mode().Perm()
		if err := os.WriteFile(r.outputPath, r.mach, 0600); err != nil {
			return result, err
		}
		result.Stdout = "Process (42) : example\nMach port raw evidence\n"
		result.Stderr = r.machStderr
		if r.cancel != nil {
			r.cancel()
		}
		return result, r.machErr
	case "/usr/sbin/lsof":
		result.Stdout = r.lsof
		result.Stderr = r.lsofStderr
		return result, r.lsofErr
	default:
		return result, errors.New("unexpected command " + name)
	}
	return result, nil
}

func TestAnalyzeCommandsAndCleanup(t *testing.T) {
	for _, full := range []bool{false, true} {
		runner := &fakeRunner{mach: machFixture(t), lsof: "p42\x00\nf3\x00au\x00tunix\x00n/tmp/socket\x00\n"}
		report, err := Analyze(context.Background(), Options{Query: "42", Full: full, Runner: runner})
		if err != nil {
			t.Fatal(err)
		}
		if report.MachPorts.Status != "ok" || report.LocalIPC.Status != "ok" || len(report.LocalIPC.Files) != 1 {
			t.Fatalf("report = %#v", report)
		}
		if len(runner.calls) != 3 {
			t.Fatalf("calls = %#v", runner.calls)
		}
		if !reflect.DeepEqual(runner.calls[1], []string{"/usr/bin/lsmp", "-p", "42", "-j", runner.outputPath}) || !reflect.DeepEqual(runner.calls[2], []string{"/usr/sbin/lsof", "-nP", "-a", "-p", "42", "-F0pftan"}) {
			t.Fatalf("calls = %#v", runner.calls)
		}
		if runner.outputMode != 0600 {
			t.Fatalf("temporary evidence mode = %o", runner.outputMode)
		}
		if _, err := os.Stat(runner.outputPath); !os.IsNotExist(err) {
			t.Fatalf("temporary output remains: %v", err)
		}
		if full && (len(report.Raw) != 2 || report.Raw[0].Stdout == "") || !full && report.Raw != nil {
			t.Fatalf("full=%v raw=%#v", full, report.Raw)
		}
	}
}

func TestAnalyzeSourceFailures(t *testing.T) {
	for _, tt := range []struct {
		name, machStatus, localStatus string
		runner                        fakeRunner
	}{
		{name: "Mach denied with local IPC", machStatus: "unavailable", localStatus: "ok", runner: fakeRunner{machErr: errors.New("exit 1"), machStderr: "task_for_pid() failed", lsof: "p42\x00\n"}},
		{name: "Mach warning", machStatus: "partial", localStatus: "ok", runner: fakeRunner{mach: machFixture(t), machStderr: "warning: run as root for cross-references", lsof: "p42\x00\n"}},
		{name: "invalid Mach JSON", machStatus: "unavailable", localStatus: "ok", runner: fakeRunner{mach: []byte("{"), lsof: "p42\x00\n"}},
		{name: "lsof inaccessible", machStatus: "ok", localStatus: "unavailable", runner: fakeRunner{mach: machFixture(t), lsofErr: errors.New("exit 1"), lsofStderr: "permission denied"}},
		{name: "partial lsof rows", machStatus: "ok", localStatus: "partial", runner: fakeRunner{mach: machFixture(t), lsof: "p42\x00\nf3\x00tPIPE\x00n->0x123\x00\n", lsofErr: errors.New("exit 1"), lsofStderr: "some descriptors unavailable"}},
		{name: "lsof warning", machStatus: "ok", localStatus: "partial", runner: fakeRunner{mach: machFixture(t), lsof: "p42\x00\n", lsofStderr: "warning: incomplete output"}},
		{name: "both failed", machStatus: "unavailable", localStatus: "unavailable", runner: fakeRunner{machErr: errors.New("lsmp missing"), lsofErr: errors.New("lsof missing")}},
		{name: "empty successful sources", machStatus: "unavailable", localStatus: "unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report, err := Analyze(context.Background(), Options{Query: "42", Full: true, Runner: &tt.runner})
			if err != nil {
				t.Fatal(err)
			}
			if report.MachPorts.Status != tt.machStatus || report.LocalIPC.Status != tt.localStatus || report.MachPorts.Ports == nil || report.LocalIPC.Files == nil {
				t.Fatalf("report = %#v", report)
			}
			if tt.machStatus != "ok" && len(report.MachPorts.Warnings) == 0 || tt.localStatus != "ok" && len(report.LocalIPC.Warnings) == 0 {
				t.Fatal("failed/partial source needs an explanation")
			}
			if _, err := os.Stat(tt.runner.outputPath); !os.IsNotExist(err) {
				t.Fatalf("temporary output remains after failure: %v", err)
			}
		})
	}
}

func TestAnalyzeTargetAndCancellation(t *testing.T) {
	for _, query := range []string{"", " ", "0", "000", "999999999999999999999999", "-1", "--help"} {
		runner := &fakeRunner{}
		if _, err := Analyze(context.Background(), Options{Query: query, Runner: runner}); err == nil || len(runner.calls) != 0 {
			t.Errorf("query=%q must be rejected before executing commands", query)
		}
	}
	runner := &fakeRunner{mach: machFixture(t), lsof: "p42\x00\n"}
	if _, err := Analyze(context.Background(), Options{Query: "example", Runner: runner}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.calls[0], []string{"pgrep", "-n", "-x", "example"}) {
		t.Fatalf("name lookup = %v", runner.calls[0])
	}
	runner = &fakeRunner{lookupErr: errors.New("process exited")}
	if _, err := Analyze(context.Background(), Options{Query: "42", Runner: runner}); err == nil || len(runner.calls) != 1 {
		t.Fatalf("lookup failure should stop collection: err=%v calls=%v", err, runner.calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runner = &fakeRunner{cancel: cancel}
	if _, err := Analyze(ctx, Options{Query: "42", Runner: runner}); !errors.Is(err, context.Canceled) || len(runner.calls) != 2 {
		t.Fatalf("cancel error=%v calls=%v", err, runner.calls)
	}
	if _, err := os.Stat(runner.outputPath); !os.IsNotExist(err) {
		t.Fatalf("temporary output remains after cancellation: %v", err)
	}
	if _, err := Analyze(ctx, Options{Query: "42", Runner: &fakeRunner{}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("already canceled: %v", err)
	}
}

func TestAnalyzeUnsupportedPlatform(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("non-macOS platform check")
	}
	_, err := Analyze(context.Background(), Options{Query: "42"})
	if err == nil || !strings.Contains(err.Error(), "requires macOS") {
		t.Fatalf("error = %v", err)
	}
}
