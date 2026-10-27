package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jdefrancesco/macscope/internal/output"
)

// Settle the current selection directly; no timer sleeps are needed in tests.
func settleSelection(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	return send(t, m, selectionReadyMsg{index: m.cursor, id: m.selectionID})
}

func TestInitialAndChangedSelectionsAutoRun(t *testing.T) {
	m, initial := send(t, New(context.Background(), testEntries()), tea.WindowSizeMsg{Width: 120, Height: 40})
	if initial == nil {
		t.Fatal("initial selection was not scheduled")
	}
	m, cmd := settleSelection(t, m)
	if !m.isRunning(0) {
		t.Fatal("initial read-only selection did not run")
	}
	m, _ = send(t, m, finished(t, cmd))
	m, _ = send(t, m, key("down"))
	m, cmd = settleSelection(t, m)
	if !m.isRunning(1) {
		t.Fatal("moving to a selection did not run it")
	}
	m, _ = send(t, m, finished(t, cmd), key("up"))
	m, cmd = settleSelection(t, m)
	if cmd != nil || m.active != nil || !strings.Contains(m.View(), "args=") {
		t.Fatal("cached selection should render without rerunning")
	}
}

func TestFastNavigationAndStaleSelectionIgnored(t *testing.T) {
	m := newSized(t)
	stale := selectionReadyMsg{index: m.cursor, id: m.selectionID}
	m, _ = send(t, m, key("down"), key("down"))
	m, cmd := send(t, m, stale)
	if cmd != nil || m.active != nil || m.mode != modeBrowse {
		t.Fatal("stale selection started a collector or prompt")
	}
	m, _ = settleSelection(t, m)
	if m.mode != modePrompt || m.active != nil {
		t.Fatal("parameter selection should open its prompt automatically")
	}
	m, _ = send(t, m, key("down"))
	if m.cursor != 3 || m.mode != modeBrowse {
		t.Fatal("empty automatic prompt blocked list navigation")
	}
}

func TestAutoLoadWhileReportFocused(t *testing.T) {
	m, _ := send(t, newSized(t), key("down"), key("tab"))
	m, cmd := settleSelection(t, m)
	if m.focus != focusReport || !m.isRunning(1) {
		t.Fatal("Tab prevented the selected report from loading")
	}
	m, _ = send(t, m, finished(t, cmd))
}

func TestAutoPromptWaitsForListFocus(t *testing.T) {
	m, _ := send(t, newSized(t), key("down"), key("down"), key("tab"))
	m, cmd := settleSelection(t, m)
	if cmd != nil || m.mode != modeBrowse {
		t.Fatal("automatic prompt stole report focus")
	}
	m, _ = send(t, m, key("tab"))
	m, _ = settleSelection(t, m)
	if m.mode != modePrompt {
		t.Fatal("returning to list did not open the selected input prompt")
	}
}

func TestAutoSelectionStopsPreviousCollector(t *testing.T) {
	wait := Entry{Name: "watch", Run: func(ctx context.Context, _ []string, _ output.Streams) error { <-ctx.Done(); return ctx.Err() }}
	m, _ := send(t, New(context.Background(), []Entry{wait, {Name: "next", Run: echoRun}}), tea.WindowSizeMsg{Width: 100, Height: 24})
	m, old := settleSelection(t, m)
	m, _ = send(t, m, key("down"))
	m, _ = settleSelection(t, m)
	if m.active == nil || !m.active.canceling {
		t.Fatal("previous collector was not canceled")
	}
	m, scheduled := send(t, m, finished(t, old))
	if scheduled == nil || m.active != nil {
		t.Fatal("new selection was not queued after cancellation")
	}
	m, next := settleSelection(t, m)
	if !m.isRunning(1) {
		t.Fatal("new selection did not run after old collector stopped")
	}
	m, _ = send(t, m, finished(t, next))
}

func TestAutoSelectionNeverStartsExportAndEscStopsPending(t *testing.T) {
	export := Entry{Name: "collect", AlwaysPrompt: true, Prompts: []PromptField{{Label: "pid"}}, Run: echoRun}
	m, _ := send(t, New(context.Background(), []Entry{export, {Name: "read", Run: echoRun}}), tea.WindowSizeMsg{Width: 100, Height: 24})
	m, cmd := settleSelection(t, m)
	if cmd != nil || m.mode != modeBrowse || m.active != nil {
		t.Fatal("selection automatically started evidence export")
	}
	m, _ = send(t, m, key("down"))
	pending := selectionReadyMsg{index: m.cursor, id: m.selectionID}
	m, _ = send(t, m, key("esc"))
	m, cmd = send(t, m, pending)
	if cmd != nil || m.active != nil {
		t.Fatal("Esc did not cancel pending automatic collection")
	}
}

func TestAutomaticSelectionRetriesCanceledResult(t *testing.T) {
	m := newSized(t)
	m.results[0] = result{canceled: true, args: []string{"previous-argument"}}
	m, cmd := settleSelection(t, m)
	if !m.isRunning(0) {
		t.Fatal("canceled cached result prevented automatic collection")
	}
	done := finished(t, cmd)
	if len(done.args) != 1 || done.args[0] != "previous-argument" {
		t.Fatal("retry lost prior arguments")
	}
	m, _ = send(t, m, done)
}

func TestSelectionWaitsForExplicitExport(t *testing.T) {
	entries := []Entry{{Name: "collect", AlwaysPrompt: true, Run: echoRun}, {Name: "next", Run: echoRun}}
	m, _ := send(t, New(context.Background(), entries), tea.WindowSizeMsg{Width: 100, Height: 24})
	m, export := send(t, m, key("enter"))
	m, _ = send(t, m, key("down"))
	m, _ = settleSelection(t, m)
	if m.active == nil || m.active.canceling {
		t.Fatal("navigation canceled explicit evidence writing")
	}
	m, _ = send(t, m, finished(t, export))
	m, next := settleSelection(t, m)
	if !m.isRunning(1) {
		t.Fatal("next selection did not start after export finished")
	}
	m, _ = send(t, m, finished(t, next))
}

func TestReturnWhilePreviousRunIsStopping(t *testing.T) {
	wait := Entry{Name: "wait", Run: func(ctx context.Context, _ []string, _ output.Streams) error { <-ctx.Done(); return ctx.Err() }}
	m, _ := send(t, New(context.Background(), []Entry{wait, {Name: "other", Run: echoRun}}), tea.WindowSizeMsg{Width: 100, Height: 24})
	m, old := settleSelection(t, m)
	m, _ = send(t, m, key("down"))
	m, _ = settleSelection(t, m)
	m, _ = send(t, m, key("up"))
	m, _ = settleSelection(t, m)
	if !m.autoPending || !m.active.canceling {
		t.Fatal("return lost the pending retry")
	}
	m, _ = send(t, m, finished(t, old))
	m, retry := settleSelection(t, m)
	if !m.isRunning(0) || m.active.canceling {
		t.Fatal("selection did not restart after old run stopped")
	}
	m, _ = send(t, m, key("esc"))
	m, _ = send(t, m, finished(t, retry))
}
