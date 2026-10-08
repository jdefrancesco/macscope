package tui

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jdefrancesco/macscope/internal/output"
)

var namedKeys = map[string]tea.KeyType{
	"enter":     tea.KeyEnter,
	"esc":       tea.KeyEsc,
	"tab":       tea.KeyTab,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"home":      tea.KeyHome,
	"end":       tea.KeyEnd,
	"pgup":      tea.KeyPgUp,
	"pgdown":    tea.KeyPgDown,
	"ctrl+c":    tea.KeyCtrlC,
	"ctrl+u":    tea.KeyCtrlU,
	"ctrl+w":    tea.KeyCtrlW,
	"backspace": tea.KeyBackspace,
	"space":     tea.KeySpace,
}

func key(s string) tea.KeyMsg {
	if t, ok := namedKeys[s]; ok {
		if t == tea.KeySpace {
			return tea.KeyMsg{Type: t, Runes: []rune{' '}}
		}
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func typeText(s string) []tea.Msg {
	msgs := make([]tea.Msg, 0, len(s))
	for _, r := range s {
		if r == ' ' {
			msgs = append(msgs, key("space"))
			continue
		}
		msgs = append(msgs, key(string(r)))
	}
	return msgs
}

// send feeds msgs through Update and returns the final model and last cmd.
func send(t *testing.T, m Model, msgs ...tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, msg := range msgs {
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(Model)
	}
	return m, cmd
}

// collect executes cmd (expanding batches) and returns every produced msg.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for _, c := range batch {
		out = append(out, collect(c)...)
	}
	return out
}

func finished(t *testing.T, cmd tea.Cmd) runFinishedMsg {
	t.Helper()
	for _, msg := range collect(cmd) {
		if done, ok := msg.(runFinishedMsg); ok {
			return done
		}
	}
	t.Fatal("cmd produced no runFinishedMsg")
	return runFinishedMsg{}
}

func isQuit(cmd tea.Cmd) bool {
	for _, msg := range collect(cmd) {
		if _, ok := msg.(tea.QuitMsg); ok {
			return true
		}
	}
	return false
}

// echoRun writes its args to Out and "warn" to Err.
func echoRun(_ context.Context, args []string, s output.Streams) error {
	fmt.Fprintf(s.Out, "args=%q\n", args)
	fmt.Fprintln(s.Err, "warn")
	return nil
}

func testEntries() []Entry {
	return []Entry{
		{Name: "specs", Summary: "hardware specs", Run: echoRun},
		{Name: "logs", Summary: "recent logs", Args: []string{"--last", "30m"}, Run: echoRun},
		{Name: "binary", Summary: "inspect a binary", Prompt: "path", Run: echoRun},
		{Name: "broken", Summary: "always fails", Run: func(context.Context, []string, output.Streams) error {
			return fmt.Errorf("boom")
		}},
	}
}

func newSized(t *testing.T) Model {
	t.Helper()
	m, _ := send(t, New(context.Background(), testEntries()), tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}
