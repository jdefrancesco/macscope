package tui

import (
	"context"
	"errors"
	"fmt"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jdefrancesco/macscope/internal/output"
	"reflect"
	"strings"
	"testing"
)

func TestMultiplePromptsAndSaveRerun(t *testing.T) {
	entry := Entry{Name: "collect", Prompts: []PromptField{{Label: "pid", Flag: "--pid", Validate: func(value string) error {
		if value != "42" {
			return errors.New("positive PID required")
		}
		return nil
	}}, {Label: "new output directory", Flag: "--out"}}, AlwaysPrompt: true, Run: echoRun}
	m, _ := send(t, New(context.Background(), []Entry{entry}), tea.WindowSizeMsg{Width: 100, Height: 24}, key("enter"))
	m, cmd := send(t, m, append(typeText("bad"), key("enter"))...)
	if cmd != nil || !strings.Contains(m.View(), "positive PID required") {
		t.Fatal("invalid PID should stay in prompt")
	}
	m, _ = send(t, m, key("ctrl+u"))
	m, cmd = send(t, m, append(typeText("42"), key("enter"))...)
	if cmd != nil || m.mode != modePrompt || !strings.Contains(m.View(), "new output directory") {
		t.Fatal("must ask destination before saving")
	}
	m, cmd = send(t, m, append(typeText("/tmp/evidence with spaces"), key("enter"))...)
	done := finished(t, cmd)
	if !reflect.DeepEqual(done.args, []string{"--pid", "42", "--out", "/tmp/evidence with spaces"}) {
		t.Fatal(done.args)
	}
	m, _ = send(t, m, done)
	m, cmd = send(t, m, key("r"))
	if cmd != nil || m.mode != modePrompt || m.promptIndex != 0 {
		t.Fatal("rerun must prompt for a new destination")
	}
	m, _ = send(t, m, key("esc"))
	if m.mode != modeBrowse || m.active != nil || len(m.promptArgs) != 0 {
		t.Fatal("canceled prompt retained pending arguments")
	}
}

func TestLiveOutputBeforeCompletionAndCancellation(t *testing.T) {
	started := make(chan struct{})
	entry := Entry{Name: "ipc watch", Command: "ipc", Args: []string{"--watch"}, Live: true, Run: func(ctx context.Context, _ []string, s output.Streams) error {
		fmt.Fprintln(s.Out, "live queue change")
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}
	m, _ := send(t, New(context.Background(), []Entry{entry}), tea.WindowSizeMsg{Width: 100, Height: 24})
	m, cmd := send(t, m, key("enter"))
	messages := make(chan []tea.Msg, 1)
	go func() { messages <- collect(cmd) }()
	<-started
	m, _ = send(t, m, spinner.TickMsg{})
	if !m.isRunning(0) || !strings.Contains(m.View(), "live queue change") || !strings.Contains(m.View(), "macscope ipc --watch") {
		t.Fatalf("live output missing: %s", m.View())
	}
	m, _ = send(t, m, key("esc"))
	m, _ = send(t, m, <-messages...)
	if m.active != nil || !m.results[0].canceled || !strings.Contains(m.View(), "live queue change") {
		t.Fatal("cancel lost live evidence")
	}
}

func TestLiveCaptureKeepsLatestAndRuneBoundaries(t *testing.T) {
	b := newLimitedBuffer(5)
	b.tail = true
	b.Write([]byte("old-prefix"))
	b.Write([]byte("▸tail"))
	c := b.snapshot()
	if !c.truncated() || !c.tail || c.text != "tail" {
		t.Fatalf("tail=%#v", c)
	}
}
