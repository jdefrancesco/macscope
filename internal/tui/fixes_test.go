package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello\nworld", "hello\nworld"},
		{"keeps SGR", "\x1b[1;38;5;63mhi\x1b[0m", "\x1b[1;38;5;63mhi\x1b[0m"},
		{"keeps bare SGR reset", "a\x1b[mb", "a\x1b[mb"},
		{"drops OSC 52 BEL", "a\x1b]52;c;Zm9v\x07b", "ab"},
		{"drops OSC ST", "a\x1b]0;title\x1b\\b", "ab"},
		{"drops DCS", "a\x1bPq#0\x1b\\b", "ab"},
		{"drops CSI cursor move", "a\x1b[2J\x1b[10;5Hb", "ab"},
		{"drops private CSI", "a\x1b[?25lb", "ab"},
		{"drops CSI with intermediate", "a\x1b[1 qb", "ab"},
		{"drops carriage return", "progress 10%\rprogress 100%\r\n", "progress 10%progress 100%\n"},
		{"drops BEL", "ding\a", "ding"},
		{"drops C1 controls", "a\u009b31mb\u0085", "a31mb"},
		{"drops DEL and C0", "a\x7f\x00\x08b", "ab"},
		{"drops two-byte escape", "a\x1b7b\x1bc", "ab"},
		{"unterminated OSC", "a\x1b]52;c;Zm9v", "a"},
		{"unterminated CSI", "a\x1b[12", "a"},
		{"trailing escape", "a\x1b", "a"},
		{"malformed CSI", "a\x1b[1\x01b", "ab"},
		{"tab expansion", "a\tb\n\tc", "a       b\n        c"},
		{"unicode kept", "héllo ▸", "héllo ▸"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitize(tt.in); got != tt.want {
				t.Fatalf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLimitedBuffer(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		writes    []string
		wantText  string
		wantTotal int64
	}{
		{"under limit", 10, []string{"abc", "def"}, "abcdef", 6},
		{"at limit", 3, []string{"abc"}, "abc", 3},
		{"over limit", 4, []string{"abc", "def", "ghi"}, "abcd", 9},
		{"cut mid rune", 4, []string{"abc▸"}, "abc", 6},
		{"full rune at edge", 5, []string{"ab▸x"}, "ab▸", 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newLimitedBuffer(tt.limit)
			for _, w := range tt.writes {
				if n, err := b.Write([]byte(w)); n != len(w) || err != nil {
					t.Fatalf("Write = %d, %v", n, err)
				}
			}
			got := b.snapshot()
			if got.text != tt.wantText || got.total != tt.wantTotal {
				t.Fatalf("snapshot = %q/%d, want %q/%d", got.text, got.total, tt.wantText, tt.wantTotal)
			}
			if got.truncated() != (tt.wantTotal > int64(len(tt.wantText))) {
				t.Fatal("truncated() mismatch")
			}
		})
	}
}

func TestLargeOutputTruncatedWithNotice(t *testing.T) {
	big := Entry{Name: "big", Run: func(_ context.Context, _ []string, s output.Streams) error {
		_, err := s.Out.Write(bytes.Repeat([]byte("x"), maxOutputBytes+10))
		return err
	}}
	m, _ := send(t, New(context.Background(), []Entry{big}), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := send(t, m, key("enter"))
	done := finished(t, cmd)
	if len(done.out.text) != maxOutputBytes || done.out.total != maxOutputBytes+10 {
		t.Fatalf("captured %d/%d bytes", len(done.out.text), done.out.total)
	}
	m, _ = send(t, m, done, key("tab"), key("G"))
	if !strings.Contains(m.View(), "run `macscope big` directly") {
		t.Fatal("truncation notice not shown at bottom of report")
	}
}

// killedErr mimics collect.Runner's *CommandError on context cancel.
var killedErr = errors.New("signal: killed")

func TestCancelDetectedFromContextNotError(t *testing.T) {
	started := make(chan struct{})
	entry := Entry{Name: "logs", Run: func(ctx context.Context, _ []string, _ output.Streams) error {
		close(started)
		<-ctx.Done()
		return killedErr
	}}
	m, _ := send(t, New(context.Background(), []Entry{entry}), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := send(t, m, key("enter"))
	msgs := make(chan []tea.Msg, 1)
	go func() { msgs <- collect(cmd) }()
	<-started
	m, _ = send(t, m, key("esc"))
	if !strings.Contains(m.View(), "canceling…") {
		t.Fatal("header should show canceling… after esc")
	}
	m, _ = send(t, m, <-msgs...)
	view := m.View()
	if !m.results[0].canceled || !strings.Contains(view, "canceled") || strings.Contains(view, "failed") {
		t.Fatalf("expected canceled status, got:\n%s", view)
	}
}

func TestWaitForActive(t *testing.T) {
	closed := make(chan struct{})
	close(closed)
	open := make(chan struct{})
	noop := func() {}
	tests := []struct {
		name  string
		final tea.Model
		want  bool
	}{
		{"nil model", nil, true},
		{"idle model", New(context.Background(), nil), true},
		{"finished run", Model{active: &activeRun{cancel: noop, done: closed}}, true},
		{"stuck run times out", Model{active: &activeRun{cancel: noop, done: open}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waitForActive(tt.final, 10*time.Millisecond); got != tt.want {
				t.Fatalf("waitForActive = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunWaitsForInFlightCommand(t *testing.T) {
	started := make(chan struct{})
	var exited atomic.Bool
	entry := Entry{Name: "slow", Run: func(ctx context.Context, _ []string, _ output.Streams) error {
		close(started)
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond) // simulate child process teardown
		exited.Store(true)
		return ctx.Err()
	}}
	in, w := io.Pipe()
	errs := make(chan error, 1)
	go func() {
		errs <- Run(context.Background(), []Entry{entry}, output.Streams{In: in, Out: io.Discard, Err: io.Discard})
	}()
	fmt.Fprint(w, "\r")
	<-started
	fmt.Fprint(w, "q")
	if err := <-errs; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !exited.Load() {
		t.Fatal("Run returned before the in-flight command exited")
	}
	w.Close()
}

func TestTooSmallTerminal(t *testing.T) {
	tests := []struct {
		w, h      int
		wantSmall bool
	}{
		{0, 0, true},
		{10, 5, true},
		{minTermWidth - 1, 40, true},
		{120, minTermHeight - 1, true},
		{minTermWidth, minTermHeight, false},
		{120, 40, false},
	}
	for _, tt := range tests {
		m, _ := send(t, New(context.Background(), testEntries()), tea.WindowSizeMsg{Width: tt.w, Height: tt.h})
		view := m.View()
		if got := strings.Contains(view, "terminal too small"); got != tt.wantSmall {
			t.Errorf("%dx%d: too-small message = %v, want %v", tt.w, tt.h, got, tt.wantSmall)
		}
		if tt.wantSmall {
			continue
		}
		if lines := strings.Count(view, "\n") + 1; lines > tt.h {
			t.Errorf("%dx%d: view has %d lines", tt.w, tt.h, lines)
		}
	}
}

func TestPromptRejectsFlagLikeValue(t *testing.T) {
	m, _ := send(t, newSized(t), key("down"), key("down"), key("enter"))
	m, cmd := send(t, m, append(typeText("--json"), key("enter"))...)
	if cmd != nil || m.active != nil || m.mode != modePrompt {
		t.Fatal("flag-like value must not run and must stay in prompt mode")
	}
	if !strings.Contains(m.View(), errFlagValue) {
		t.Fatal("inline rejection message not shown")
	}
	m, _ = send(t, m, key("ctrl+u"))
	if m.promptErr != "" {
		t.Fatal("editing should clear the rejection message")
	}
}

func TestLineInputStripsControls(t *testing.T) {
	paste := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/a\nb\tc\x1b\r"), Paste: true}
	if got := (lineInput{}).Update(paste).Value(); got != "/tmp/abc" {
		t.Fatalf("Value() = %q, want /tmp/abc", got)
	}
}

func TestReportFocusTopBottomKeys(t *testing.T) {
	long := Entry{Name: "long", Run: func(_ context.Context, _ []string, s output.Streams) error {
		_, err := s.Out.Write([]byte(strings.Repeat("line\n", 200)))
		return err
	}}
	m, _ := send(t, New(context.Background(), []Entry{long, long}), tea.WindowSizeMsg{Width: 80, Height: 20})
	m, cmd := send(t, m, key("enter"))
	m, _ = send(t, m, finished(t, cmd), key("tab"))
	for _, k := range []struct{ bottom, top string }{{"G", "g"}, {"end", "home"}} {
		m, _ = send(t, m, key(k.bottom))
		if !m.viewport.AtBottom() || m.cursor != 0 {
			t.Fatalf("%s should scroll report to bottom without moving cursor", k.bottom)
		}
		m, _ = send(t, m, key(k.top))
		if !m.viewport.AtTop() {
			t.Fatalf("%s should scroll report to top", k.top)
		}
	}
}

func TestWrapCache(t *testing.T) {
	m, cmd := send(t, newSized(t), key("enter"))
	m, _ = send(t, m, finished(t, cmd))
	if _, ok := m.wrapped[0]; !ok {
		t.Fatal("finished result should be cached")
	}
	m, _ = send(t, m, key("down"), key("up"))
	if len(m.wrapped) != 1 {
		t.Fatalf("placeholder content must not be cached: %d entries", len(m.wrapped))
	}
	before := m.wrapped
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.wrapWidth == 0 || len(before) != 1 {
		t.Fatal("resize should rebuild the cache without mutating the old map")
	}
	m, cmd = send(t, m, key("r"))
	if _, ok := m.wrapped[0]; !ok {
		t.Fatal("cache kept while rerun is in flight")
	}
	m, _ = send(t, m, finished(t, cmd))
	if len(before) != 1 {
		t.Fatal("old cache map was mutated")
	}
}
