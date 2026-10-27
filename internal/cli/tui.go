package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"

	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/process"
	"github.com/jdefrancesco/macscope/internal/tui"
)

var errTUINotTerminal = errors.New("tui requires an interactive terminal; run individual commands (or --json) for scripted output")

// tuiPreset describes how the TUI should invoke a command: fixed leading
// arguments and an optional single prompted value.
type tuiPreset struct {
	Args         []string
	Prompt       string
	Prompts      []tui.PromptField
	AlwaysPrompt bool
	Live         bool
}

func runTUI(ctx context.Context, args []string, streams output.Streams) error {
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			printTUIHelp(streams.Out)
			return nil
		default:
			return fmt.Errorf("unknown tui argument: %s; usage: macscope tui", arg)
		}
	}
	if !isTerminal(streams.In) || !isTerminal(streams.Out) {
		return errTUINotTerminal
	}
	return tui.Run(ctx, tuiEntries(), streams)
}

// isTerminal reports whether v is an *os.File attached to a terminal.
func isTerminal(v any) bool {
	f, ok := v.(*os.File)
	if !ok || f == nil {
		return false
	}
	return term.IsTerminal(f.Fd())
}

// tuiPresets maps command names to the invocation the TUI uses. Commands not
// listed here are excluded from the browser.
func tuiPresets() map[string]tuiPreset {
	last30m := []string{"--last", "30m"}
	return map[string]tuiPreset{
		"specs":    {},
		"disk":     {},
		"version":  {},
		"persist":  {},
		"agents":   {},
		"daemons":  {},
		"sysext":   {},
		"vpn":      {},
		"tcc":      {Args: last30m},
		"es":       {Args: last30m},
		"panic":    {Args: []string{"--last"}},
		"macho":    {Prompt: "path"},
		"proc":     {Args: []string{"--maps", "--tree"}, Prompt: "pid or name"},
		"ipc":      {Prompt: "pid or name"},
		"attach":   {Prompt: "pid"},
		"timeline": {Args: []string{"--pid"}, Prompt: "pid"},
		"collect": {Prompts: []tui.PromptField{
			{Label: "pid", Flag: "--pid", Validate: func(value string) error {
				if _, ok := process.ParsePID(value); !ok {
					return errors.New("enter a positive PID")
				}
				return nil
			}},
			{Label: "new output directory (saves raw evidence locally)", Flag: "--out", Validate: func(value string) error {
				_, err := os.Lstat(value)
				if err == nil {
					return errors.New("output path already exists; choose a new directory")
				}
				if !os.IsNotExist(err) {
					return err
				}
				return nil
			}},
		}, AlwaysPrompt: true, Live: true},
	}
}

// tuiEntries builds the browser entries from commands(), preserving order.
func tuiEntries() []tui.Entry {
	presets := tuiPresets()
	var entries []tui.Entry
	for _, cmd := range commands() {
		preset, ok := presets[cmd.Name]
		if !ok {
			continue
		}
		entries = append(entries, tui.Entry{
			Name:    cmd.Name,
			Summary: cmd.Summary,
			Args:    append([]string(nil), preset.Args...),
			Prompt:  preset.Prompt,
			Prompts: preset.Prompts, AlwaysPrompt: preset.AlwaysPrompt, Live: preset.Live,
			Run: cmd.Run,
		})
		if cmd.Name == "ipc" {
			entries = append(entries, tui.Entry{Name: "ipc watch", Command: "ipc", Summary: "Watch IPC changes live; Esc stops collection.", Args: []string{"--watch"}, Prompt: "pid or name", Run: runIPC, Live: true})
		}
	}
	return entries
}

func printTUIHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope tui")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Open an interactive terminal browser: highlight a command to load it automatically.")
	fmt.Fprintln(w, "A short pause while navigating avoids running every row. Cached results appear immediately.")
	fmt.Fprintln(w, "PID/path selections open their input prompt automatically; r refreshes a report.")
	fmt.Fprintln(w, "A spinner appears while the command")
	fmt.Fprintln(w, "collects, then scroll the colored results. IPC watch updates live; Esc stops it.")
	fmt.Fprintln(w, "The collect entry prompts for a PID and a new output directory before saving raw")
	fmt.Fprintln(w, "evidence locally. Other entries are read-only. The TUI runs the")
	fmt.Fprintln(w, "same collectors as the plain commands.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Keys:")
	fmt.Fprintln(w, "  ↑/↓ auto-load • enter run/input • r refresh • tab focus • pgup/pgdn scroll • esc cancel • q quit")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Plain and --json output of individual commands is unchanged for scripting.")
}
