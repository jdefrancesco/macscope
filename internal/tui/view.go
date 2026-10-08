package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	appName    = "macscope"
	appTagline = "macOS introspection & triage"
	keyHelp    = "↑/↓ select • enter run • r rerun • tab focus • pgup/pgdn scroll • esc cancel • q quit"
	loadingMsg = "loading…"
	listTitle  = "Commands"

	listTitleHeight = 1
)

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return loadingMsg
	}
	if m.tooSmall() {
		return fmt.Sprintf("terminal too small (%dx%d); resize to at least %dx%d",
			m.width, m.height, minTermWidth, minTermHeight)
	}
	l := m.layout()
	panes := lipgloss.JoinHorizontal(lipgloss.Top, m.listPane(l), m.reportPane(l))
	return lipgloss.JoinVertical(lipgloss.Left,
		m.fit(m.titleBar()),
		panes,
		m.fit(m.statusLine()),
		m.fit(m.styles.muted.Render(keyHelp)),
	)
}

// fit truncates a single line to the terminal width.
func (m Model) fit(line string) string {
	return lipgloss.NewStyle().MaxWidth(max(m.width, 1)).Render(line)
}

func (m Model) titleBar() string {
	return m.styles.title.Render(appName) + "  " + m.styles.subtitle.Render(appTagline)
}

func (m Model) paneStyle(f focus, width, height int) lipgloss.Style {
	style := m.styles.paneBlurred
	if m.focus == f {
		style = m.styles.paneFocused
	}
	return style.Width(max(width-borderSize, 1)).Height(max(height-borderSize, 1))
}

func (m Model) listPane(l layout) string {
	inner := max(l.listWidth-borderSize, 1)
	rows := max(l.paneHeight-borderSize-listTitleHeight, 1)
	start := max(0, m.cursor-rows+1)
	end := min(len(m.entries), start+rows)
	lines := make([]string, 0, end-start+listTitleHeight)
	lines = append(lines, lipgloss.NewStyle().MaxWidth(inner).Render(m.styles.paneTitle.Render(listTitle)))
	for i := start; i < end; i++ {
		lines = append(lines, m.listItem(i, inner))
	}
	return m.paneStyle(focusList, l.listWidth, l.paneHeight).Render(strings.Join(lines, "\n"))
}

func (m Model) listItem(index, width int) string {
	marker, style := " ", m.styles.item
	if index == m.cursor {
		marker, style = cursorMarker, m.styles.itemSelected
	}
	line := style.Render(marker + " " + m.entries[index].Name)
	if m.isRunning(index) {
		line += " " + m.spinner.View()
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

func (m Model) reportPane(l layout) string {
	body := m.viewport.View()
	if m.isRunning(m.cursor) {
		body = m.spinner.View() + " " + m.styles.muted.Render("running…")
	}
	width, _ := l.viewportSize()
	header := lipgloss.NewStyle().MaxWidth(width).Render(m.reportHeader())
	content := header + "\n\n" + body
	return m.paneStyle(focusReport, l.reportWidth, l.paneHeight).
		Padding(0, reportPadX).
		Render(content)
}

func (m Model) reportHeader() string {
	entry, ok := m.selected()
	if !ok {
		return ""
	}
	args, status := m.headerState()
	return m.styles.commandLine.Render(entry.CommandLine(args...)) + "  " + status
}

// headerState returns the extra args to display and a styled status label.
func (m Model) headerState() ([]string, string) {
	entry, _ := m.selected()
	if m.isRunning(m.cursor) {
		label := "running " + formatElapsed(time.Since(m.active.started))
		if m.active.canceling {
			label = "canceling…"
		}
		return extraArgs(entry, m.active.args), m.styles.warn.Render(label)
	}
	res, ok := m.results[m.cursor]
	if !ok {
		return nil, m.styles.muted.Render("not run")
	}
	label, style := "ok", m.styles.ok
	switch {
	case res.canceled:
		label, style = "canceled", m.styles.warn
	case res.err != nil:
		label, style = "failed", m.styles.danger
	}
	return extraArgs(entry, res.args), style.Render(label + " " + formatElapsed(res.elapsed))
}

// extraArgs strips the entry's own Args prefix so CommandLine does not
// repeat them.
func extraArgs(entry Entry, args []string) []string {
	if len(args) < len(entry.Args) {
		return args
	}
	for i, a := range entry.Args {
		if args[i] != a {
			return args
		}
	}
	return args[len(entry.Args):]
}

func (m Model) statusLine() string {
	if m.mode == modePrompt {
		entry, _ := m.selected()
		line := m.styles.promptLabel.Render(entry.Prompt+" "+cursorMarker) + " " + m.input.View()
		if m.promptErr != "" {
			line += "  " + m.styles.danger.Render(m.promptErr)
		}
		return line
	}
	entry, ok := m.selected()
	if !ok {
		return ""
	}
	return entry.Summary
}

func formatElapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
