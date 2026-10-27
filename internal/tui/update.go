package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// errFlagValue explains why prompted values may not look like flags: the
// command handlers do not support "--" to end option parsing.
const errFlagValue = `values starting with "-" are not allowed here`

const selectionDelay = 200 * time.Millisecond

type selectionReadyMsg struct{ index, id int }

func (m Model) selectionCmd() tea.Cmd {
	index, id := m.cursor, m.selectionID
	return tea.Tick(selectionDelay, func(time.Time) tea.Msg { return selectionReadyMsg{index: index, id: id} })
}

func (m Model) queueSelection() (Model, tea.Cmd) {
	m.selectionID++
	m.autoPending = true
	return m, m.selectionCmd()
}

func (m Model) autoSelect(msg selectionReadyMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.selectionID || msg.index != m.cursor || !m.autoPending {
		return m, nil
	}
	if m.mode != modeBrowse || m.tooSmall() || m.ctx.Err() != nil {
		m.autoPending = false
		return m, nil
	}
	entry, ok := m.selected()
	if !ok {
		m.autoPending = false
		return m, nil
	}
	if m.active != nil {
		if m.active.index == m.cursor {
			// A return to a selection whose old run is still stopping should
			// restart it only after that run has actually exited.
			if !m.active.canceling {
				m.autoPending = false
			}
			return m, nil
		}
		// Explicit evidence export is allowed to finish writing its manifest.
		if !m.entries[m.active.index].AlwaysPrompt {
			m = m.cancelActive()
		}
		return m, nil
	}
	m.autoPending = false
	if entry.AlwaysPrompt {
		return m, nil
	}
	if last, cached := m.results[m.cursor]; cached {
		if last.canceled {
			return m.startRun(m.cursor, append([]string{}, last.args...))
		}
		return m.refreshReport(), nil
	}
	if len(entry.promptFields()) > 0 {
		if m.focus != focusList {
			return m, nil
		}
		m.autoPrompt = true
		return m.enterPrompt()
	}
	return m.startRun(m.cursor, entryArgs(entry))
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		initial, wasSmall := !m.ready, m.tooSmall()
		m = m.resize(msg.Width, msg.Height)
		if len(m.entries) > 0 && !m.tooSmall() && (initial || wasSmall) {
			return m.queueSelection()
		}
		return m, nil
	case selectionReadyMsg:
		return m.autoSelect(msg)
	case runFinishedMsg:
		valid := m.active != nil && msg.runID == m.active.id
		m = m.finishRun(msg)
		if valid && m.autoPending {
			return m, m.selectionCmd()
		}
		return m, nil
	case spinner.TickMsg:
		if m.active == nil {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		previous := m.results[m.active.index]
		if m.active.live && m.active.out != nil && (m.active.out.byteCount() != previous.out.total || m.active.errOut.byteCount() != previous.errOut.total) {
			atBottom := m.viewport.AtBottom()
			m.results = withResult(m.results, m.active.index, result{args: m.active.args, out: m.active.out.snapshot(), errOut: m.active.errOut.snapshot()})
			m.wrapped = withoutKey(m.wrapped, m.active.index)
			if m.cursor == m.active.index {
				m = m.setReportContent(false)
				if atBottom {
					m.viewport.GotoBottom()
				}
			}
		}
		return m, cmd
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m.quit()
	}
	if m.mode == modePrompt {
		return m.handlePromptKey(msg)
	}
	return m.handleBrowseKey(msg)
}

func (m Model) handleBrowseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.quit()
	case "tab", "shift+tab":
		m = m.toggleFocus()
		if m.focus == focusList && m.active == nil {
			return m.queueSelection()
		}
		return m, nil
	case "esc":
		m.autoPending = false
		m.selectionID++
		return m.cancelActive(), nil
	case "enter":
		return m.activate()
	case "r":
		return m.rerun()
	case "pgup", "pgdown":
		return m.scrollReport(msg)
	}
	if m.focus == focusReport {
		return m.handleReportKey(msg)
	}
	previous := m.cursor
	m = m.navigate(msg.String())
	if m.cursor != previous {
		return m.queueSelection()
	}
	return m, nil
}

func (m Model) handleReportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "g", "home":
		m.viewport.GotoTop()
		return m, nil
	case "G", "end":
		m.viewport.GotoBottom()
		return m, nil
	}
	return m.scrollReport(msg)
}

func (m Model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.autoPrompt && m.input.Value() == "" {
		switch msg.String() {
		case "up", "down", "home", "end":
			m = m.leavePrompt()
			return m.handleBrowseKey(msg)
		}
	}
	switch msg.String() {
	case "esc":
		m.autoPending = false
		m.selectionID++
		return m.leavePrompt(), nil
	case "enter":
		return m.submitPrompt()
	}
	m.input = m.input.Update(msg)
	m.promptErr = ""
	return m, nil
}

// submitPrompt validates the prompted value and runs it as one argument.
func (m Model) submitPrompt() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}
	if strings.HasPrefix(value, "-") {
		m.promptErr = errFlagValue
		return m, nil
	}
	entry, _ := m.selected()
	fields := entry.promptFields()
	if m.promptIndex >= len(fields) {
		return m.leavePrompt(), nil
	}
	field := fields[m.promptIndex]
	if field.Validate != nil {
		if err := field.Validate(value); err != nil {
			m.promptErr = err.Error()
			return m, nil
		}
	}
	args := append([]string{}, m.promptArgs...)
	if field.Flag != "" {
		args = append(args, field.Flag)
	}
	args = append(args, value)
	if m.promptIndex+1 < len(fields) {
		m.promptArgs = args
		m.promptIndex++
		m.input = lineInput{}
		m.promptErr = ""
		return m, nil
	}
	return m.leavePrompt().startRun(m.cursor, args)
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	return m.cancelActive(), tea.Quit
}

// cancelActive cancels the running command's context; the run still reports
// back through runFinishedMsg.
func (m Model) cancelActive() Model {
	if m.active == nil {
		return m
	}
	m.active.cancel()
	next := *m.active
	next.canceling = true
	m.active = &next
	return m
}

func (m Model) toggleFocus() Model {
	if m.focus == focusList {
		m.focus = focusReport
	} else {
		m.focus = focusList
	}
	return m
}

func (m Model) scrollReport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) navigate(key string) Model {
	next := m.cursor
	switch key {
	case "up", "k":
		next--
	case "down", "j":
		next++
	case "home", "g":
		next = 0
	case "end", "G":
		next = len(m.entries) - 1
	default:
		return m
	}
	return m.moveCursor(next)
}

func (m Model) moveCursor(next int) Model {
	next = clamp(next, 0, len(m.entries)-1)
	if next == m.cursor {
		return m
	}
	m.cursor = next
	return m.refreshReport()
}

// activate runs the selected entry or opens its prompt.
func (m Model) activate() (tea.Model, tea.Cmd) {
	entry, ok := m.selected()
	if !ok || m.active != nil {
		return m, nil
	}
	m.autoPending = false
	m.selectionID++
	m.autoPrompt = false
	if len(entry.promptFields()) > 0 {
		return m.enterPrompt()
	}
	return m.startRun(m.cursor, entryArgs(entry))
}

// rerun repeats the selected entry with the args of its last result.
func (m Model) rerun() (tea.Model, tea.Cmd) {
	if m.active != nil {
		return m, nil
	}
	m.autoPending = false
	m.selectionID++
	m.autoPrompt = false
	if entry, ok := m.selected(); ok && entry.AlwaysPrompt {
		return m.activate()
	}
	last, ok := m.results[m.cursor]
	if !ok {
		return m.activate()
	}
	return m.startRun(m.cursor, append([]string(nil), last.args...))
}

func (m Model) enterPrompt() (tea.Model, tea.Cmd) {
	m.mode = modePrompt
	m.input = lineInput{}
	m.promptIndex = 0
	m.promptArgs = append([]string{}, m.entries[m.cursor].Args...)
	m.promptErr = ""
	return m, nil
}

func (m Model) leavePrompt() Model {
	m.mode = modeBrowse
	m.input = lineInput{}
	m.promptErr = ""
	m.promptIndex = 0
	m.promptArgs = nil
	m.autoPrompt = false
	return m
}
