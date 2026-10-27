package proctriage

import (
	"context"
	"errors"
	"github.com/jdefrancesco/macscope/internal/collect"
	"os"
	"strings"
	"testing"
)

type procRunner struct {
	denyIdentity, denyTree, changed bool
	identities                      int
	calls                           [][]string
}

func (r *procRunner) Run(_ context.Context, name string, args ...string) (collect.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	result := collect.Result{}
	switch name {
	case "ps":
		result.Stdout = "42 20 alice staff S /Applications/Example /Applications/Example App.app/Contents/MacOS/Example App\n"
	case "/bin/ps":
		if args[0] == "-axww" {
			if r.denyTree {
				return result, errors.New("denied")
			}
			data, err := os.ReadFile("../../testdata/process/tree.txt")
			result.Stdout = string(data)
			return result, err
		}
		if r.denyIdentity {
			return result, errors.New("denied")
		}
		r.identities++
		start := "09:00:00"
		if r.changed && r.identities > 1 {
			start = "09:00:01"
		}
		result.Stdout = "42 20 Mon Oct 26 " + start + " 2026 /Applications/Example App.app/Contents/MacOS/Example App\n"
	case "/usr/bin/vmmap":
		data, err := os.ReadFile("../../testdata/memory/vmmap.txt")
		result.Stdout = string(data)
		return result, err
	case "codesign":
		result.Stdout = "Identifier=example\n"
		if args[0] == "--verify" {
			result.ExitCode = 1
			result.Stderr = "invalid signature"
		}
	default:
		return result, errors.New("unexpected command")
	}
	return result, nil
}

func TestAnalyzeRuntimeOptions(t *testing.T) {
	for _, tt := range []struct {
		name     string
		runner   procRunner
		warnings bool
	}{
		{"complete", procRunner{}, false},
		{"identity inaccessible", procRunner{denyIdentity: true}, true},
		{"tree inaccessible", procRunner{denyTree: true}, true},
		{"PID changed", procRunner{changed: true}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &tt.runner
			report, err := AnalyzeWithOptions(context.Background(), Options{Query: "42", Maps: true, Tree: true, Full: true, Runner: r})
			if err != nil {
				t.Fatal(err)
			}
			if (len(report.Warnings) > 0) != tt.warnings || report.Maps == nil || report.Maps.Raw == nil || report.Tree == nil {
				t.Fatalf("report=%#v", report)
			}
			if r.denyIdentity {
				if report.Process.Path != "" {
					t.Fatal("retained ambiguous path")
				}
				for _, call := range r.calls {
					if call[0] == "codesign" {
						t.Fatal("signed an ambiguous path")
					}
				}
			} else {
				if !strings.Contains(report.Process.Path, "Example App") || len(report.Findings) != 1 {
					t.Fatalf("report=%#v", report)
				}
				for _, call := range r.calls {
					if call[0] == "codesign" && call[len(call)-1] != report.Process.Path {
						t.Fatal("path was split")
					}
				}
			}
		})
	}
}

func TestAnalyzeInvalidTargetAndCanceledContext(t *testing.T) {
	r := &procRunner{}
	if _, err := AnalyzeWithOptions(context.Background(), Options{Query: "0", Runner: r}); err == nil || len(r.calls) != 0 {
		t.Fatal("invalid PID reached collectors")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AnalyzeWithOptions(ctx, Options{Query: "42", Runner: r}); !errors.Is(err, context.Canceled) || len(r.calls) != 0 {
		t.Fatalf("canceled request reached collectors: %v", err)
	}
}
