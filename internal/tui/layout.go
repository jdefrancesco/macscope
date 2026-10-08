package tui

import (
	"github.com/charmbracelet/lipgloss"
)

const (
	titleHeight        = 1
	footerHeight       = 2
	borderSize         = 2 // one cell on each side
	reportPadX         = 1
	reportHeaderHeight = 2 // command line + blank separator
	listDecoration     = 6 // marker, spaces, running indicator
	minViewportWidth   = 1
	minViewportHeight  = 1
	minListInnerWidth  = 4
	minTermWidth       = 40
	minTermHeight      = 10
)

// layout holds outer pane dimensions derived from the terminal size.
type layout struct {
	listWidth   int
	reportWidth int
	paneHeight  int
}

func (m Model) layout() layout {
	listInner := clamp(m.longestName()+listDecoration, minListInnerWidth, max(m.width/2, minListInnerWidth))
	listWidth := listInner + borderSize
	minReport := borderSize + 2*reportPadX + minViewportWidth
	minPane := borderSize + reportHeaderHeight + minViewportHeight
	return layout{
		listWidth:   listWidth,
		reportWidth: max(m.width-listWidth, minReport),
		paneHeight:  max(m.height-titleHeight-footerHeight, minPane),
	}
}

func (l layout) viewportSize() (int, int) {
	w := l.reportWidth - borderSize - 2*reportPadX
	h := l.paneHeight - borderSize - reportHeaderHeight
	return max(w, minViewportWidth), max(h, minViewportHeight)
}

func (m Model) longestName() int {
	longest := 0
	for _, e := range m.entries {
		longest = max(longest, lipgloss.Width(e.Name))
	}
	return longest
}

func (m Model) resize(width, height int) Model {
	m.width = max(width, 0)
	m.height = max(height, 0)
	m.ready = true
	m.viewport.Width, m.viewport.Height = m.layout().viewportSize()
	return m.setReportContent(false)
}

// refreshReport shows the selected entry's content scrolled to the top.
func (m Model) refreshReport() Model {
	return m.setReportContent(true)
}

func (m Model) setReportContent(resetScroll bool) Model {
	if m.wrapWidth != m.viewport.Width {
		m.wrapWidth = m.viewport.Width
		m.wrapped = map[int]string{}
	}
	content, cached := m.wrapped[m.cursor]
	if !cached {
		content = m.wrap(m.reportContent())
		if m.hasResult(m.cursor) {
			m.wrapped = withKey(m.wrapped, m.cursor, content)
		}
	}
	m.viewport.SetContent(content)
	if resetScroll {
		m.viewport.GotoTop()
	}
	return m
}

func (m Model) wrap(content string) string {
	return lipgloss.NewStyle().Width(m.viewport.Width).Render(content)
}

// hasResult reports whether index shows a cached (cacheable) result.
func (m Model) hasResult(index int) bool {
	_, ok := m.results[index]
	return ok && !m.isRunning(index)
}

// tooSmall reports whether the terminal cannot fit the two-pane layout.
func (m Model) tooSmall() bool {
	return m.width < minTermWidth || m.height < minTermHeight
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return min(max(v, lo), hi)
}
