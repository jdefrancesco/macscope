package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jdefrancesco/macscope/internal/output"
)

func TestCursorNavigation(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want int
	}{
		{"starts at top", nil, 0},
		{"down", []string{"down"}, 1},
		{"j", []string{"j", "j"}, 2},
		{"clamped at bottom", []string{"down", "down", "down", "down", "down"}, 3},
		{"clamped at top", []string{"up", "k"}, 0},
		{"end", []string{"end"}, 3},
		{"G then g", []string{"G", "g"}, 0},
		{"home", []string{"down", "home"}, 0},
		{"unknown key ignored", []string{"x"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newSized(t)
			for _, k := range tt.keys {
				m, _ = send(t, m, key(k))
			}
			if m.cursor != tt.want {
				t.Fatalf("cursor = %d, want %d", m.cursor, tt.want)
			}
		})
	}
}

func TestEnterRunsEntryWithoutPrompt(t *testing.T) {
	m, _ := send(t, newSized(t), key("down"))
	m, cmd := send(t, m, key("enter"))
	if !m.isRunning(1) {
		t.Fatal("expected entry 1 to be running")
	}
	if !strings.Contains(m.View(), "running") {
		t.Fatal("view should show running state")
	}
	done := finished(t, cmd)
	if want := []string{"--last", "30m"}; !reflect.DeepEqual(done.args, want) {
		t.Fatalf("args = %q, want %q", done.args, want)
	}
	if !strings.Contains(done.out.text, `args=["--last" "30m"]`) || done.errOut.text != "warn\n" {
		t.Fatalf("unexpected captured output %q / %q", done.out, done.errOut)
	}
	m, _ = send(t, m, done)
	if m.active != nil {
		t.Fatal("run should be finished")
	}
	view := m.View()
	for _, want := range []string{"macscope logs --last 30m", "warn", "ok"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestEnterIgnoredWhileRunning(t *testing.T) {
	m, cmd := send(t, newSized(t), key("enter"))
	m, second := send(t, m, key("down"), key("enter"))
	if second != nil || m.active.index != 0 || m.cursor != 1 {
		t.Fatal("second enter should be ignored but navigation should work")
	}
	m, _ = send(t, m, finished(t, cmd))
	if _, ok := m.results[0]; !ok {
		t.Fatal("result for entry 0 should be cached")
	}
	if !strings.Contains(m.View(), "Press enter to run `macscope logs --last 30m`") {
		t.Fatal("unrun entry should show placeholder")
	}
}

func TestPromptFlowPassesSingleArg(t *testing.T) {
	m, _ := send(t, newSized(t), key("down"), key("down"), key("enter"))
	if m.mode != modePrompt {
		t.Fatal("expected prompt mode")
	}
	if !strings.Contains(m.View(), "path ▸") {
		t.Fatal("prompt label not shown")
	}
	m, _ = send(t, m, typeText("  /Applications/My App.app ")...)
	m, cmd := send(t, m, key("enter"))
	if m.mode != modeBrowse {
		t.Fatal("expected browse mode after submit")
	}
	done := finished(t, cmd)
	if want := []string{"/Applications/My App.app"}; !reflect.DeepEqual(done.args, want) {
		t.Fatalf("args = %q, want %q", done.args, want)
	}
}

func TestPromptEmptyValueIgnored(t *testing.T) {
	m, _ := send(t, newSized(t), key("end"), key("up"), key("enter"))
	m, cmd := send(t, m, append(typeText("   "), key("enter"))...)
	if cmd != nil || m.mode != modePrompt || m.active != nil {
		t.Fatal("blank prompt value must not run")
	}
}

func TestEscCancelsPrompt(t *testing.T) {
	m, _ := send(t, newSized(t), key("down"), key("down"), key("enter"))
	m, _ = send(t, m, append(typeText("abc"), key("esc"))...)
	if m.mode != modeBrowse || m.input.Value() != "" || m.active != nil {
		t.Fatal("esc should leave prompt without running")
	}
}

func TestQuitKeys(t *testing.T) {
	tests := []struct {
		name     string
		setup    []string
		press    string
		wantQuit bool
	}{
		{"q in browse", nil, "q", true},
		{"ctrl+c in browse", nil, "ctrl+c", true},
		{"q typed in prompt", []string{"down", "down", "enter"}, "q", false},
		{"ctrl+c in prompt", []string{"down", "down", "enter"}, "ctrl+c", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newSized(t)
			for _, k := range tt.setup {
				m, _ = send(t, m, key(k))
			}
			m, cmd := send(t, m, key(tt.press))
			if got := isQuit(cmd); got != tt.wantQuit {
				t.Fatalf("quit = %v, want %v", got, tt.wantQuit)
			}
			if !tt.wantQuit && m.input.Value() != "q" {
				t.Fatalf("input = %q, want q", m.input.Value())
			}
		})
	}
}

func TestStaleRunIDIgnored(t *testing.T) {
	m, cmd := send(t, newSized(t), key("enter"))
	done := finished(t, cmd)
	stale := done
	stale.runID = done.runID + 99
	m, _ = send(t, m, stale)
	if m.active == nil || len(m.results) != 0 {
		t.Fatal("stale message must be ignored")
	}
	m, _ = send(t, m, done)
	if m.active != nil || len(m.results) != 1 {
		t.Fatal("matching message must finish the run")
	}
}

func TestErrorResultRendersError(t *testing.T) {
	m, cmd := send(t, newSized(t), key("end"), key("enter"))
	m, _ = send(t, m, finished(t, cmd))
	view := m.View()
	if !strings.Contains(view, "error: boom") || !strings.Contains(view, "failed") {
		t.Fatalf("view missing error output:\n%s", view)
	}
}

func TestEscCancelsRunningCommand(t *testing.T) {
	blocking := Entry{Name: "wait", Run: func(ctx context.Context, _ []string, _ output.Streams) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	m, _ := send(t, New(context.Background(), []Entry{blocking}), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := send(t, m, key("enter"))
	m, _ = send(t, m, key("esc"))
	done := finished(t, cmd)
	if !errors.Is(done.err, context.Canceled) {
		t.Fatalf("err = %v, want canceled", done.err)
	}
	m, _ = send(t, m, done)
	if !m.results[0].canceled || !strings.Contains(m.View(), "canceled") {
		t.Fatal("canceled run should render canceled")
	}
}

func TestRerunUsesLastArgs(t *testing.T) {
	m, _ := send(t, newSized(t), key("down"), key("down"), key("enter"))
	m, cmd := send(t, m, append(typeText("/bin/ls"), key("enter"))...)
	m, _ = send(t, m, finished(t, cmd))
	m, cmd = send(t, m, key("r"))
	if m.mode != modeBrowse {
		t.Fatal("rerun should not prompt again")
	}
	if done := finished(t, cmd); !reflect.DeepEqual(done.args, []string{"/bin/ls"}) {
		t.Fatalf("rerun args = %q", done.args)
	}
}

func TestRerunWithoutResultActivates(t *testing.T) {
	m, cmd := send(t, newSized(t), key("r"))
	if cmd == nil || m.active == nil {
		t.Fatal("r without a result should run the entry")
	}
	if _, again := send(t, m, key("r")); again != nil {
		t.Fatal("r while running should be ignored")
	}
}

func TestFocusAndScrolling(t *testing.T) {
	long := strings.Repeat("line\n", 200)
	entry := Entry{Name: "long", Run: func(_ context.Context, _ []string, s output.Streams) error {
		_, err := s.Out.Write([]byte(long))
		return err
	}}
	m, _ := send(t, New(context.Background(), []Entry{entry, entry}), tea.WindowSizeMsg{Width: 80, Height: 20})
	m, cmd := send(t, m, key("enter"))
	m, _ = send(t, m, finished(t, cmd))
	m, _ = send(t, m, key("pgdown"))
	if m.viewport.YOffset == 0 {
		t.Fatal("pgdown should scroll even with list focus")
	}
	m, _ = send(t, m, key("tab"), key("g"))
	if m.focus != focusReport || m.cursor != 0 {
		t.Fatal("tab should focus report; keys go to viewport")
	}
	m, _ = send(t, m, key("j"))
	if m.cursor != 0 {
		t.Fatal("j in report focus must not move the list cursor")
	}
	m, _ = send(t, m, key("tab"), key("j"), key("k"))
	if m.focus != focusList || m.viewport.YOffset != 0 {
		t.Fatal("cursor change should reset scroll to top")
	}
}

func TestSpinnerTickOnlyWhileRunning(t *testing.T) {
	m := newSized(t)
	if _, cmd := send(t, m, m.spinner.Tick()); cmd != nil {
		t.Fatal("idle spinner tick should stop")
	}
	m, _ = send(t, m, key("enter"))
	if _, cmd := send(t, m, m.spinner.Tick()); cmd == nil {
		t.Fatal("running spinner should keep ticking")
	}
}

func TestNilRunReportsError(t *testing.T) {
	m, _ := send(t, New(nil, []Entry{{Name: "none"}}), tea.WindowSizeMsg{Width: 80, Height: 24}) //nolint:staticcheck // nil ctx is handled
	m, cmd := send(t, m, key("enter"))
	if done := finished(t, cmd); done.err == nil {
		t.Fatal("nil Run should produce an error")
	}
	if m.Init() != nil {
		t.Fatal("Init should return nil")
	}
}
