package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Streams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

func (s Streams) WithDefaults() Streams {
	if s.In == nil {
		s.In = os.Stdin
	}
	if s.Out == nil {
		s.Out = os.Stdout
	}
	if s.Err == nil {
		s.Err = os.Stderr
	}
	return s
}

type TextWriter struct {
	w io.Writer
}

var (
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	keyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	bulletStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	okStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	warnStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	dangerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func NewTextWriter(w io.Writer) TextWriter {
	return TextWriter{w: w}
}

func (tw TextWriter) Section(title string) error {
	_, err := fmt.Fprintf(tw.w, "%s\n", colorize(sectionStyle, title+":"))
	return err
}

func (tw TextWriter) KeyValue(key, value string) error {
	label := fmt.Sprintf("%-24s", key+":")
	_, err := fmt.Fprintf(tw.w, "  %s %s\n", colorize(keyStyle, label), value)
	return err
}

func (tw TextWriter) Bullet(value string) error {
	_, err := fmt.Fprintf(tw.w, "  %s %s\n", colorize(bulletStyle, "-"), value)
	return err
}

func Level(level string) string {
	switch strings.ToUpper(level) {
	case "LOW":
		return colorize(okStyle, level)
	case "MODERATE", "MEDIUM":
		return colorize(warnStyle, level)
	case "HIGH", "CRITICAL":
		return colorize(dangerStyle, level)
	default:
		return colorize(mutedStyle, level)
	}
}

func Muted(value string) string {
	return colorize(mutedStyle, value)
}

func colorize(style lipgloss.Style, value string) string {
	if noColor() {
		return value
	}
	return style.Render(value)
}

func noColor() bool {
	return os.Getenv("NO_COLOR") != "" ||
		os.Getenv("MACSCOPE_NO_COLOR") != "" ||
		os.Getenv("TERM") == "dumb"
}

func WriteJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
