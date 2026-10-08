package tui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestViewBeforeSizeShowsLoading(t *testing.T) {
	if got := New(context.Background(), testEntries()).View(); got != loadingMsg {
		t.Fatalf("View() = %q, want %q", got, loadingMsg)
	}
}

func TestViewSizes(t *testing.T) {
	sizes := []struct{ w, h int }{{0, 0}, {1, 1}, {10, 5}, {20, 8}, {80, 24}, {120, 40}}
	for _, sz := range sizes {
		m, _ := send(t, New(context.Background(), testEntries()), tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		m, cmd := send(t, m, key("enter"))
		_ = m.View()
		m, _ = send(t, m, finished(t, cmd))
		if view := m.View(); view == "" {
			t.Fatalf("empty view at %dx%d", sz.w, sz.h)
		}
	}
}

func TestViewLayout120x40(t *testing.T) {
	m := newSized(t)
	view := m.View()
	for _, want := range []string{appName, appTagline, "Commands", keyHelp, "hardware specs", "▸ specs", "not run"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > 40 {
		t.Errorf("view has %d lines, want <= 40", lines)
	}
}

func TestPlaceholderMentionsPrompt(t *testing.T) {
	m, _ := send(t, newSized(t), key("down"), key("down"))
	if !strings.Contains(m.View(), "asked for a path") {
		t.Fatal("placeholder should mention the prompt")
	}
}

func TestViewEmptyEntries(t *testing.T) {
	m, _ := send(t, New(context.Background(), nil), tea.WindowSizeMsg{Width: 40, Height: 10}, key("enter"), key("down"))
	if !strings.Contains(m.View(), "No commands") {
		t.Fatal("expected empty-state message")
	}
}

func TestRenderResult(t *testing.T) {
	m := New(context.Background(), nil)
	tests := []struct {
		name string
		res  result
		want []string
	}{
		{"stdout only", result{out: captured{text: "hello\n", total: 6}}, []string{"hello"}},
		{"stderr appended", result{out: captured{text: "a", total: 1}, errOut: captured{text: "b", total: 1}}, []string{"a", "b"}},
		{"truncated", result{args: []string{"x"}, out: captured{text: "abc", total: 9}}, []string{"abc", "output truncated (9 bytes); run `macscope t x` directly"}},
		{"error sanitized", result{err: errors.New("bad\x1b]52;c;Zm9v\x07")}, []string{"error: bad"}},
		{"error line", result{err: errors.New("x")}, []string{"error: x"}},
		{"canceled", result{err: context.Canceled, canceled: true}, []string{"canceled"}},
		{"empty", result{}, []string{"(no output)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.renderResult(Entry{Name: "t"}, tt.res)
			if strings.Contains(got, "\x1b]") {
				t.Errorf("renderResult leaked OSC: %q", got)
			}
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("renderResult = %q, missing %q", got, w)
				}
			}
		})
	}
}

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{250 * time.Millisecond, "250ms"},
		{1500 * time.Millisecond, "1.5s"},
	}
	for _, tt := range tests {
		if got := formatElapsed(tt.in); got != tt.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtraArgs(t *testing.T) {
	e := Entry{Args: []string{"--last", "30m"}}
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--last", "30m", "x"}, "x"},
		{[]string{"--last"}, "--last"},
		{[]string{"--first", "1h"}, "--first 1h"},
	}
	for _, tt := range tests {
		if got := strings.Join(extraArgs(e, tt.args), " "); got != tt.want {
			t.Errorf("extraArgs(%q) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestEntryCommandLine(t *testing.T) {
	tests := []struct {
		entry Entry
		extra []string
		want  string
	}{
		{Entry{Name: "specs"}, nil, "macscope specs"},
		{Entry{Name: "logs", Args: []string{"--last", "30m"}}, nil, "macscope logs --last 30m"},
		{Entry{Name: "binary"}, []string{"/bin/ls"}, "macscope binary /bin/ls"},
	}
	for _, tt := range tests {
		if got := tt.entry.CommandLine(tt.extra...); got != tt.want {
			t.Errorf("CommandLine = %q, want %q", got, tt.want)
		}
	}
}

func TestNewStylesNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if colorEnabledFromEnv() {
		t.Fatal("NO_COLOR should disable color")
	}
	st := newStyles(false)
	if got := st.danger.Render("x"); got != "x" {
		t.Fatalf("no-color style rendered %q", got)
	}
}

func TestRunRejectsEmptyEntries(t *testing.T) {
	if err := Run(context.Background(), nil, output.Streams{}); !errors.Is(err, errNoEntries) {
		t.Fatalf("err = %v, want errNoEntries", err)
	}
}

func TestRunQuitsOnQ(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out bytes.Buffer
	streams := output.Streams{In: strings.NewReader("q"), Out: &out, Err: &bytes.Buffer{}}
	if err := Run(ctx, testEntries(), streams); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunWrapsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	blockingIn, _ := newBlockingReader()
	streams := output.Streams{In: blockingIn, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := Run(ctx, testEntries(), streams)
	if err == nil || !strings.HasPrefix(err.Error(), "tui:") {
		t.Fatalf("err = %v, want wrapped tui error", err)
	}
}
