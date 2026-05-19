package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandSmokeHelpAndCompletions(t *testing.T) {
	bin := buildMacscopeBinary(t)
	repoRoot := repoRoot(t)

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "help",
			args: []string{"help"},
			want: []string{"macscope <command> [flags]", "macscope completion <bash|zsh|fish>"},
		},
		{
			name: "version",
			args: []string{"version"},
			want: []string{"macscope dev"},
		},
		{
			name: "macho help",
			args: []string{"macho", "--help"},
			want: []string{"macscope macho [--json] [--full] [--triage] <path>"},
		},
		{
			name: "proc help",
			args: []string{"proc", "--help"},
			want: []string{"macscope proc [--json] <pid-or-name>"},
		},
		{
			name: "attach help",
			args: []string{"attach", "--help"},
			want: []string{"macscope attach [--json] [--last 30m] <pid>"},
		},
		{
			name: "persist help",
			args: []string{"persist", "--help"},
			want: []string{"macscope persist [--json] [--dir <launchd-dir>]"},
		},
		{
			name: "tcc help",
			args: []string{"tcc", "--help"},
			want: []string{"macscope tcc [--json] [--last 30m]"},
		},
		{
			name: "es help",
			args: []string{"es", "--help"},
			want: []string{"macscope es [--json] [--last 30m]"},
		},
		{
			name: "vpn help",
			args: []string{"vpn", "--help"},
			want: []string{"macscope vpn [--json] [--last 60m] [vpn-name]"},
		},
		{
			name: "panic help",
			args: []string{"panic", "--help"},
			want: []string{"macscope panic --file <panic-file> [--json]"},
		},
		{
			name: "timeline help",
			args: []string{"timeline", "--help"},
			want: []string{"macscope timeline --pid <pid> [--last 30m] [--json|--jsonl]"},
		},
		{
			name: "completion help",
			args: []string{"completion", "--help"},
			want: []string{"macscope completion <bash|zsh|fish>"},
		},
		{
			name: "bash completion",
			args: []string{"completion", "bash"},
			want: []string{"complete -F _macscope macscope", "--json --full --triage"},
		},
		{
			name: "zsh completion",
			args: []string{"completion", "zsh"},
			want: []string{"#compdef macscope", "'macscope commands'"},
		},
		{
			name: "fish completion",
			args: []string{"completion", "fish"},
			want: []string{"complete -c macscope -f", "__fish_seen_subcommand_from completion"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runMacscope(t, bin, repoRoot, tt.args...)
			if err != nil {
				t.Fatalf("macscope %s failed: %v\nstderr:\n%s\nstdout:\n%s", strings.Join(tt.args, " "), err, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout missing %q:\n%s", want, stdout)
				}
			}
		})
	}
}

func TestCommandSmokeFixtureCommands(t *testing.T) {
	bin := buildMacscopeBinary(t)
	repoRoot := repoRoot(t)

	tests := []struct {
		name string
		args []string
		want []string
		json bool
	}{
		{
			name: "panic fixture human",
			args: []string{"panic", "--file", "testdata/panic/watchdog.panic"},
			want: []string{"Panic Type:", "WATCHDOG_TIMEOUT", "92 seconds"},
		},
		{
			name: "panic fixture json",
			args: []string{"panic", "--json", "--file", "testdata/panic/watchdog.panic"},
			want: []string{`"panic_type": "WATCHDOG_TIMEOUT"`},
			json: true,
		},
		{
			name: "persist fixture human",
			args: []string{"persist", "--dir", "testdata/launchd"},
			want: []string{"Launchd Persistence:", "USER_WRITABLE_PERSISTENCE", "com.example.bad"},
		},
		{
			name: "persist fixture json",
			args: []string{"persist", "--json", "--dir", "testdata/launchd"},
			want: []string{`"label": "com.example.bad"`},
			json: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runMacscope(t, bin, repoRoot, tt.args...)
			if err != nil {
				t.Fatalf("macscope %s failed: %v\nstderr:\n%s\nstdout:\n%s", strings.Join(tt.args, " "), err, stderr, stdout)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if tt.json {
				var decoded any
				if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
					t.Fatalf("invalid JSON: %v\n%s", err, stdout)
				}
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout missing %q:\n%s", want, stdout)
				}
			}
		})
	}
}

func TestCommandSmokeCompletionShellSyntax(t *testing.T) {
	bin := buildMacscopeBinary(t)
	repoRoot := repoRoot(t)

	tests := []struct {
		shell string
		bin   string
		args  []string
	}{
		{shell: "bash", bin: "bash", args: []string{"-n"}},
		{shell: "zsh", bin: "zsh", args: []string{"-n"}},
		{shell: "fish", bin: "fish", args: []string{"-n"}},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			shellBin, err := exec.LookPath(tt.bin)
			if err != nil {
				t.Skipf("%s not installed", tt.bin)
			}

			stdout, stderr, err := runMacscope(t, bin, repoRoot, "completion", tt.shell)
			if err != nil {
				t.Fatalf("macscope completion %s failed: %v\nstderr:\n%s\nstdout:\n%s", tt.shell, err, stderr, stdout)
			}

			script := filepath.Join(t.TempDir(), "macscope."+tt.shell)
			if err := os.WriteFile(script, []byte(stdout), 0644); err != nil {
				t.Fatal(err)
			}

			args := append(tt.args, script)
			cmd := exec.Command(shellBin, args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s syntax check failed: %v\n%s", tt.bin, err, string(output))
			}
		})
	}
}

func buildMacscopeBinary(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "macscope")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "MACSCOPE_NO_COLOR=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}
	return bin
}

func runMacscope(t *testing.T, bin, dir string, args ...string) (string, string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "MACSCOPE_NO_COLOR=1")

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		err = ctx.Err()
	}
	return stdout.String(), stderr.String(), err
}

func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
