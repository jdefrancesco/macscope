package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type mode int

const (
	modeBrowse mode = iota
	modePrompt
)

type focus int

const (
	focusList focus = iota
	focusReport
)

// result is the cached outcome of one finished run.
type result struct {
	args     []string
	out      captured
	errOut   captured
	err      error
	canceled bool
	elapsed  time.Duration
}

// activeRun describes the single in-flight command. It is replaced, never
// mutated, once created.
type activeRun struct {
	index     int
	id        int
	args      []string
	cancel    context.CancelFunc
	done      <-chan struct{}
	started   time.Time
	canceling bool
}

// Model is the Bubble Tea model for the macscope command browser.
type Model struct {
	ctx       context.Context
	entries   []Entry
	cursor    int
	mode      mode
	focus     focus
	results   map[int]result
	wrapped   map[int]string // wrapped result content at wrapWidth
	wrapWidth int
	promptErr string
	active    *activeRun
	nextRunID int
	width     int
	height    int
	ready     bool
	viewport  viewport.Model
	input     lineInput
	spinner   spinner.Model
	styles    styles
}

var _ tea.Model = Model{}

// New builds a browser model over entries. Commands run with child contexts
// of ctx.
func New(ctx context.Context, entries []Entry) Model {
	if ctx == nil {
		ctx = context.Background()
	}
	st := newStyles(colorEnabledFromEnv())
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(st.spinnerAccent))
	return Model{
		ctx:      ctx,
		entries:  append([]Entry(nil), entries...),
		results:  map[int]result{},
		wrapped:  map[int]string{},
		viewport: viewport.New(minViewportWidth, minViewportHeight),
		spinner:  sp,
		styles:   st,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) selected() (Entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return Entry{}, false
	}
	return m.entries[m.cursor], true
}

func (m Model) isRunning(index int) bool {
	return m.active != nil && m.active.index == index
}

// withResult returns a copy of results with index set to r.
func withResult(results map[int]result, index int, r result) map[int]result {
	return withKey(results, index, r)
}

// withKey returns a copy of src with k set to v.
func withKey[V any](src map[int]V, k int, v V) map[int]V {
	next := make(map[int]V, len(src)+1)
	for key, val := range src {
		next[key] = val
	}
	next[k] = v
	return next
}

// withoutKey returns a copy of src without k.
func withoutKey[V any](src map[int]V, k int) map[int]V {
	next := make(map[int]V, len(src))
	for key, val := range src {
		if key != k {
			next[key] = val
		}
	}
	return next
}
