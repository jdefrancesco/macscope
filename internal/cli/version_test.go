package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestRunVersionHuman(t *testing.T) {
	withVersionMetadata(t, "v1.2.3", "abc1234", "2026-05-19T12:00:00Z")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"version"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(version) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	for _, want := range []string{
		"macscope v1.2.3",
		"Commit:",
		"abc1234",
		"Date:",
		"2026-05-19T12:00:00Z",
		"Platform:",
		runtime.GOOS + "/" + runtime.GOARCH,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunVersionJSON(t *testing.T) {
	withVersionMetadata(t, "v1.2.3", "abc1234", "2026-05-19T12:00:00Z")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"version", "--json"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(version --json) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var got versionInfo
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got.Name != "macscope" || got.Version != "v1.2.3" || got.Commit != "abc1234" || got.Date != "2026-05-19T12:00:00Z" {
		t.Fatalf("version info = %#v, want injected metadata", got)
	}
	if got.GoVersion == "" || got.GOOS == "" || got.GOARCH == "" {
		t.Fatalf("version info missing runtime metadata: %#v", got)
	}
}

func TestRunTopLevelVersionShortcut(t *testing.T) {
	withVersionMetadata(t, "v1.2.3", "abc1234", "2026-05-19T12:00:00Z")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--version"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(--version) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope v1.2.3") {
		t.Fatalf("stdout = %q, want version", stdout.String())
	}
}

func TestRunTopLevelVersionShortcutJSON(t *testing.T) {
	withVersionMetadata(t, "v1.2.3", "abc1234", "2026-05-19T12:00:00Z")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--version", "--json"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(--version --json) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var got versionInfo
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got.Version != "v1.2.3" || got.Commit != "abc1234" {
		t.Fatalf("version info = %#v, want injected metadata", got)
	}
}

func withVersionMetadata(t *testing.T, wantVersion, wantCommit, wantDate string) {
	t.Helper()

	oldVersion := version
	oldCommit := buildCommit
	oldDate := buildDate
	version = wantVersion
	buildCommit = wantCommit
	buildDate = wantDate
	t.Cleanup(func() {
		version = oldVersion
		buildCommit = oldCommit
		buildDate = oldDate
	})
}
