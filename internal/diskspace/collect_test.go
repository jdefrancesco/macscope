package diskspace

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type fakeRunner struct {
	output string
	stderr string
	err    error
	call   []string
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) (collect.Result, error) {
	r.call = append([]string{name}, args...)
	if err := ctx.Err(); err != nil {
		return collect.Result{}, err
	}
	return collect.Result{Stdout: r.output, Stderr: r.stderr}, r.err
}

func TestAnalyze(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts Options
		rows int
	}{
		{"default excludes helpers and virtual mounts", Options{}, 3},
		{"all keeps local mounts", Options{All: true}, 6},
		{"explicit path keeps its volume", Options{Path: "-disk with spaces"}, 6},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{output: dfFixture(t)}
			tt.opts.Runner = runner
			got, err := Analyze(context.Background(), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Volumes) != tt.rows {
				t.Fatalf("volumes = %#v, want %d rows", got.Volumes, tt.rows)
			}
			want := []string{"/bin/df", "-k", "-P", "-I"}
			if tt.opts.Path == "" {
				want = append(want, "-l")
			} else {
				path, err := filepath.Abs(tt.opts.Path)
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, path)
				if got.RequestedPath != path {
					t.Fatalf("requested path = %q, want %q", got.RequestedPath, path)
				}
			}
			if !reflect.DeepEqual(runner.call, want) {
				t.Fatalf("call = %#v, want %#v", runner.call, want)
			}
		})
	}
}

func TestAnalyzeError(t *testing.T) {
	_, err := Analyze(context.Background(), Options{Runner: &fakeRunner{err: errors.New("exited with code 1"), stderr: "df: path: No such file or directory"}})
	if err == nil || !strings.Contains(err.Error(), "No such file or directory") {
		t.Fatalf("error = %v, want actionable df error", err)
	}
	_, err = Analyze(context.Background(), Options{Runner: &fakeRunner{output: "unparseable"}})
	if err == nil {
		t.Fatal("expected parse error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Analyze(ctx, Options{Runner: &fakeRunner{}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
