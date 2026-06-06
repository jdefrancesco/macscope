package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestRunCompletionHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"completion", "--help"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 0 {
		t.Fatalf("Run(completion --help) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "macscope completion <bash|zsh|fish>") {
		t.Fatalf("stdout = %q, want completion usage", stdout.String())
	}
}

func TestCompletionScripts(t *testing.T) {
	tests := []struct {
		shell string
		want  []string
	}{
		{
			shell: "bash",
			want: []string{
				"complete -F _macscope macscope",
				"macho proc attach persist tcc es vpn panic timeline sysext completion",
				"--json --full --triage",
			},
		},
		{
			shell: "zsh",
			want: []string{
				"#compdef macscope",
				"'macscope commands'",
				"'macho:Inspect binary/app identity",
				"'1:shell:(bash zsh fish)'",
			},
		},
		{
			shell: "fish",
			want: []string{
				"complete -c macscope -f",
				"complete -c macscope -n '__fish_use_subcommand' -a 'macho'",
				"complete -c macscope -n '__fish_seen_subcommand_from completion' -a 'fish'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			got, err := completionScript(tt.shell)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("%s completion missing %q:\n%s", tt.shell, want, got)
				}
			}
		})
	}
}

func TestRunCompletionUnsupportedShell(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"completion", "csh"}, output.Streams{
		Out: &stdout,
		Err: &stderr,
	})

	if code != 1 {
		t.Fatalf("Run(completion csh) exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `completion: unsupported shell "csh"`) {
		t.Fatalf("stderr = %q, want unsupported shell", stderr.String())
	}
}
