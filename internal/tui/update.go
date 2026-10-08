package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// errFlagValue explains why prompted values may not look like flags: the
// command handlers do not support "--" to end option parsing.
const errFlagValue = `values starting with "-" are not allowed here`

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg.Width, msg.Height), nil
	case runFinishedMsg:
		return m.finishRun(msg), nil
	case spinner.TickMsg:
		if m.active == nil {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
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
		return m.toggleFocus(), nil
	case "esc":
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
	return m.navigate(msg.String()), nil
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
	switch msg.String() {
	case "esc":
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
	return m.leavePrompt().startRun(m.cursor, entryArgs(entry, value))
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
	if entry.Prompt != "" {
		return m.enterPrompt()
	}
	return m.startRun(m.cursor, entryArgs(entry))
}

// rerun repeats the selected entry with the args of its last result.
func (m Model) rerun() (tea.Model, tea.Cmd) {
	if m.active != nil {
		return m, nil
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
	return m, nil
}

func (m Model) leavePrompt() Model {
	m.mode = modeBrowse
	m.input = lineInput{}
	m.promptErr = ""
	return m
}
