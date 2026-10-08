package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jdefrancesco/macscope/internal/output"
)

// shutdownWait bounds how long Run waits for an in-flight command to exit
// after its context is canceled, so child processes are not orphaned.
const shutdownWait = 2 * time.Second

// runFinishedMsg reports the outcome of one command run.
type runFinishedMsg struct {
	index    int
	runID    int
	args     []string
	out      captured
	errOut   captured
	err      error
	canceled bool
	elapsed  time.Duration
}

var errNoEntries = errors.New("tui: no commands to browse")

// Run starts the interactive browser and blocks until the user quits or ctx
// is canceled.
func Run(ctx context.Context, entries []Entry, streams output.Streams) error {
	if len(entries) == 0 {
		return errNoEntries
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	streams = streams.WithDefaults()
	program := tea.NewProgram(
		New(ctx, entries),
		tea.WithAltScreen(),
		tea.WithContext(ctx),
		tea.WithInput(streams.In),
		tea.WithOutput(streams.Out),
	)
	final, err := program.Run()
	cancel()
	waitForActive(final, shutdownWait)
	if err != nil {
		return fmt.Errorf("tui: run browser: %w", err)
	}
	return nil
}

// waitForActive blocks until final's in-flight run (if any) returns, or until
// timeout elapses.
func waitForActive(final tea.Model, timeout time.Duration) bool {
	m, ok := final.(Model)
	if !ok || m.active == nil {
		return true
	}
	m.active.cancel()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-m.active.done:
		return true
	case <-timer.C:
		return false
	}
}

// runEntryCmd executes entry.Run off the UI goroutine with captured streams.
// done is closed once entry.Run has returned.
func runEntryCmd(ctx context.Context, entry Entry, index, runID int, args []string, done chan<- struct{}) tea.Cmd {
	return func() tea.Msg {
		defer close(done)
		out, errOut := newLimitedBuffer(maxOutputBytes), newLimitedBuffer(maxOutputBytes)
		streams := output.Streams{In: strings.NewReader(""), Out: out, Err: errOut}
		start := time.Now()
		err := invoke(ctx, entry, args, streams)
		return runFinishedMsg{
			index:    index,
			runID:    runID,
			args:     args,
			out:      out.snapshot(),
			errOut:   errOut.snapshot(),
			err:      err,
			canceled: ctx.Err() != nil,
			elapsed:  time.Since(start),
		}
	}
}

func invoke(ctx context.Context, entry Entry, args []string, streams output.Streams) error {
	if entry.Run == nil {
		return fmt.Errorf("command %q has no runner", entry.Name)
	}
	return entry.Run(ctx, args, streams)
}

// startRun launches entry index with args unless a run is already active.
func (m Model) startRun(index int, args []string) (Model, tea.Cmd) {
	if m.active != nil || index < 0 || index >= len(m.entries) {
		return m, nil
	}
	runCtx, cancel := context.WithCancel(m.ctx)
	done := make(chan struct{})
	m.nextRunID++
	m.active = &activeRun{
		index:   index,
		id:      m.nextRunID,
		args:    args,
		cancel:  cancel,
		done:    done,
		started: time.Now(),
	}
	cmd := runEntryCmd(runCtx, m.entries[index], index, m.nextRunID, args, done)
	return m.refreshReport(), tea.Batch(cmd, m.spinner.Tick)
}

// finishRun records a finished run, ignoring stale messages.
func (m Model) finishRun(msg runFinishedMsg) Model {
	if m.active == nil || msg.runID != m.active.id {
		return m
	}
	m.active.cancel()
	m.active = nil
	m.results = withResult(m.results, msg.index, result{
		args:     msg.args,
		out:      msg.out,
		errOut:   msg.errOut,
		err:      msg.err,
		canceled: msg.canceled,
		elapsed:  msg.elapsed,
	})
	m.wrapped = withoutKey(m.wrapped, msg.index)
	if msg.index != m.cursor {
		return m
	}
	return m.refreshReport()
}

// entryArgs returns a fresh slice of entry.Args plus extra values.
func entryArgs(entry Entry, extra ...string) []string {
	args := make([]string, 0, len(entry.Args)+len(extra))
	args = append(args, entry.Args...)
	return append(args, extra...)
}
