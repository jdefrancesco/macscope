package tui

import (
	"fmt"
	"strings"
)

// reportContent renders the body for the selected entry.
func (m Model) reportContent() string {
	entry, ok := m.selected()
	if !ok {
		return m.styles.muted.Render("No commands available.")
	}
	if m.isRunning(m.cursor) {
		return ""
	}
	res, ok := m.results[m.cursor]
	if !ok {
		return m.placeholder(entry)
	}
	return m.renderResult(entry, res)
}

func (m Model) placeholder(entry Entry) string {
	text := fmt.Sprintf("Press enter to run `%s`", entry.CommandLine())
	if entry.Prompt != "" {
		text = fmt.Sprintf("Press enter to run `%s <%s>`\n\nYou will be asked for a %s first.",
			entry.CommandLine(), entry.Prompt, entry.Prompt)
	}
	return m.styles.muted.Render(text)
}

// renderResult builds sanitized stdout, muted stderr and a status line.
func (m Model) renderResult(entry Entry, res result) string {
	command := entry.CommandLine(extraArgs(entry, res.args)...)
	parts := make([]string, 0, 5)
	if out := m.renderStream(res.out, command); out != "" {
		parts = append(parts, out)
	}
	if errOut := m.renderStream(res.errOut, command); errOut != "" {
		parts = append(parts, m.styles.muted.Render(errOut))
	}
	switch {
	case res.canceled:
		parts = append(parts, m.styles.warn.Render("canceled"))
	case res.err != nil:
		parts = append(parts, m.styles.danger.Render("error: "+sanitize(res.err.Error())))
	}
	if len(parts) == 0 {
		return m.styles.muted.Render("(no output)")
	}
	return strings.Join(parts, "\n\n")
}

// renderStream sanitizes one captured stream and appends a truncation notice
// when the cap was hit.
func (m Model) renderStream(c captured, command string) string {
	text := strings.TrimRight(sanitize(c.text), "\n")
	if !c.truncated() {
		return text
	}
	notice := fmt.Sprintf("… output truncated (%d bytes); run `%s` directly for full output", c.total, command)
	return strings.TrimLeft(text+"\n"+m.styles.warn.Render(notice), "\n")
}
