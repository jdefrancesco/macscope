package tui

import (
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

const inputCursor = "█"

// lineInput is a minimal single-line text editor for prompt mode. It is a
// value type: every edit returns a new lineInput.
type lineInput struct {
	value []rune
}

func (in lineInput) Value() string {
	return string(in.value)
}

// Update applies one key press and returns the edited input.
func (in lineInput) Update(msg tea.KeyMsg) lineInput {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		return in.insert(msg.Runes)
	case tea.KeyBackspace:
		return in.truncate(len(in.value) - 1)
	case tea.KeyCtrlU:
		return lineInput{}
	case tea.KeyCtrlW:
		return in.truncate(previousWordStart(in.value))
	}
	return in
}

// insert appends runes, dropping newlines and other control characters
// (e.g. from a multi-line paste).
func (in lineInput) insert(runes []rune) lineInput {
	if len(runes) == 0 {
		runes = []rune{' '}
	}
	next := make([]rune, 0, len(in.value)+len(runes))
	next = append(next, in.value...)
	for _, r := range runes {
		if !unicode.IsControl(r) {
			next = append(next, r)
		}
	}
	return lineInput{value: next}
}

func (in lineInput) truncate(n int) lineInput {
	n = clamp(n, 0, len(in.value))
	return lineInput{value: append([]rune(nil), in.value[:n]...)}
}

// previousWordStart finds where the last word (and trailing spaces) begin.
func previousWordStart(value []rune) int {
	i := len(value)
	for i > 0 && value[i-1] == ' ' {
		i--
	}
	for i > 0 && value[i-1] != ' ' {
		i--
	}
	return i
}

func (in lineInput) View() string {
	return string(in.value) + inputCursor
}
