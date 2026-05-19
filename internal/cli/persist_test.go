package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestRunPersistHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"persist", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(persist --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope persist [--json] [--dir <launchd-dir>]") {
		t.Fatalf("stdout = %q, want persist usage", stdout.String())
	}
}

func TestRunPersistDirHuman(t *testing.T) {
	dir := writePersistFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"persist", "--dir", dir}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(persist --dir) exit code = %d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"Launchd Persistence:",
		"USER_WRITABLE_PERSISTENCE",
		"NETWORK_DOWNLOADER_PERSISTENCE",
		"com.example.bad",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunPersistDirJSON(t *testing.T) {
	dir := writePersistFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"persist", "--json", "--dir", dir}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(persist --json --dir) exit code = %d stderr=%q", code, stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(decoded["jobs"].([]any)) != 1 {
		t.Fatalf("jobs = %#v, want one job", decoded["jobs"])
	}
}

func writePersistFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "launchd", "suspicious.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "com.example.bad.plist"), data, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}
