package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Palette mirrors the colors used by internal/output.
const (
	colorAccent  = "63"
	colorKey     = "39"
	colorBorder  = "240"
	colorMuted   = "245"
	colorOK      = "42"
	colorWarn    = "214"
	colorDanger  = "203"
	cursorMarker = "▸"
)

// styles groups every lipgloss style the view uses so color can be
// switched off in one place.
type styles struct {
	title         lipgloss.Style
	subtitle      lipgloss.Style
	paneFocused   lipgloss.Style
	paneBlurred   lipgloss.Style
	paneTitle     lipgloss.Style
	itemSelected  lipgloss.Style
	item          lipgloss.Style
	commandLine   lipgloss.Style
	muted         lipgloss.Style
	ok            lipgloss.Style
	warn          lipgloss.Style
	danger        lipgloss.Style
	promptLabel   lipgloss.Style
	spinnerAccent lipgloss.Style
}

func newStyles(colorEnabled bool) styles {
	fg := func(s lipgloss.Style, color string) lipgloss.Style {
		if !colorEnabled {
			return s
		}
		return s.Foreground(lipgloss.Color(color))
	}
	pane := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	border := func(color string) lipgloss.Style {
		if !colorEnabled {
			return pane
		}
		return pane.BorderForeground(lipgloss.Color(color))
	}
	bold := lipgloss.NewStyle().Bold(true)
	plain := lipgloss.NewStyle()
	return styles{
		title:         fg(bold, colorAccent),
		subtitle:      fg(plain, colorMuted),
		paneFocused:   border(colorAccent),
		paneBlurred:   border(colorBorder),
		paneTitle:     fg(bold, colorAccent),
		itemSelected:  fg(bold, colorKey),
		item:          plain,
		commandLine:   fg(bold, colorKey),
		muted:         fg(plain, colorMuted),
		ok:            fg(bold, colorOK),
		warn:          fg(bold, colorWarn),
		danger:        fg(bold, colorDanger),
		promptLabel:   fg(bold, colorAccent),
		spinnerAccent: fg(plain, colorAccent),
	}
}

// colorEnabledFromEnv follows the same rules as internal/output.
func colorEnabledFromEnv() bool {
	return os.Getenv("NO_COLOR") == "" &&
		os.Getenv("MACSCOPE_NO_COLOR") == "" &&
		os.Getenv("TERM") != "dumb"
}
