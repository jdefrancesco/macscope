package systeminfo

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type fakeRunner struct {
	outputs map[string]string
	fail    string
	calls   [][]string
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) (collect.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if err := ctx.Err(); err != nil {
		return collect.Result{}, err
	}
	if name == r.fail {
		return collect.Result{}, errors.New("native tool failed")
	}
	return collect.Result{Stdout: r.outputs[name]}, nil
}

func TestAnalyze(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"/usr/sbin/system_profiler": fixture(t, "apple_silicon.json"),
		"/usr/sbin/sysctl":          fixture(t, "sysctl.txt"),
		"/usr/bin/sw_vers":          fixture(t, "sw_vers.txt"),
	}}
	report, err := Analyze(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if report.Hardware.MemoryBytes != 51539607552 || report.Hardware.PhysicalCores != 14 || report.OS.Build != "24F74" {
		t.Fatalf("incomplete report: %#v", report)
	}
	want := [][]string{
		{"/usr/sbin/system_profiler", "-json", "-detailLevel", "mini", "SPHardwareDataType", "SPDisplaysDataType"},
		{"/usr/sbin/sysctl", "hw.memsize", "hw.physicalcpu", "hw.logicalcpu", "hw.machine"},
		{"/usr/bin/sw_vers"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestAnalyzeFailure(t *testing.T) {
	for _, tool := range []string{"/usr/sbin/system_profiler", "/usr/sbin/sysctl", "/usr/bin/sw_vers"} {
		t.Run(tool, func(t *testing.T) {
			runner := &fakeRunner{fail: tool, outputs: map[string]string{
				"/usr/sbin/system_profiler": fixture(t, "apple_silicon.json"),
				"/usr/sbin/sysctl":          fixture(t, "sysctl.txt"),
			}}
			_, err := Analyze(context.Background(), runner)
			if err == nil || !strings.Contains(err.Error(), "native tool failed") {
				t.Fatalf("error = %v, want collection failure", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Analyze(ctx, &fakeRunner{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestAnalyzeArchitecture(t *testing.T) {
	for _, tt := range []struct {
		name, hardware, machine, native, execution string
	}{
		{"Apple Silicon native", "apple_silicon.json", "arm64", "arm64", ""},
		{"Apple Silicon under Rosetta", "apple_silicon.json", "x86_64", "arm64", "x86_64"},
		{"Intel native", "intel.json", "x86_64", "x86_64", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{outputs: map[string]string{
				"/usr/sbin/system_profiler": fixture(t, tt.hardware),
				"/usr/sbin/sysctl":          strings.ReplaceAll(fixture(t, "sysctl.txt"), "hw.machine: arm64", "hw.machine: "+tt.machine),
				"/usr/bin/sw_vers":          fixture(t, "sw_vers.txt"),
			}}
			got, err := Analyze(context.Background(), runner)
			if err != nil {
				t.Fatal(err)
			}
			if got.Hardware.Architecture != tt.native || got.Hardware.ExecutionArchitecture != tt.execution {
				t.Fatalf("native/execution architecture = %q/%q, want %q/%q", got.Hardware.Architecture, got.Hardware.ExecutionArchitecture, tt.native, tt.execution)
			}
		})
	}
}
