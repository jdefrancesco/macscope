package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/jdefrancesco/macscope/internal/output"
)

var version = "dev"
var buildCommit = "unknown"
var buildDate = "unknown"

type commandSpec struct {
	Name    string
	Usage   string
	Summary string
	Run     func(context.Context, []string, output.Streams) error
}

func Run(ctx context.Context, args []string, streams output.Streams) int {
	streams = streams.WithDefaults()

	if len(args) == 0 {
		printHelp(streams.Out)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp(streams.Out)
		return 0
	case "--version":
		if err := runVersion(ctx, args[1:], streams); err != nil {
			fmt.Fprintf(streams.Err, "version: %v\n", err)
			return 1
		}
		return 0
	}

	cmd, ok := commandByName(args[0])
	if !ok {
		fmt.Fprintf(streams.Err, "unknown command: %s\n\n", args[0])
		printUsage(streams.Err)
		return 2
	}

	if err := cmd.Run(ctx, args[1:], streams); err != nil {
		fmt.Fprintf(streams.Err, "%s: %v\n", cmd.Name, err)
		return 1
	}

	return 0
}

func commandByName(name string) (commandSpec, bool) {
	for _, cmd := range commands() {
		if cmd.Name == name {
			return cmd, true
		}
	}
	return commandSpec{}, false
}

func commands() []commandSpec {
	return []commandSpec{
		{
			Name:    "version",
			Usage:   "macscope version [--json]",
			Summary: "Show build version and platform metadata.",
			Run:     runVersion,
		},
		{
			Name:    "macho",
			Usage:   "macscope macho [--json] [--full] [--triage] <path>",
			Summary: "Inspect binary/app identity, architecture, signing, Gatekeeper, xattrs, and linked libraries.",
			Run:     runMacho,
		},
		{
			Name:    "proc",
			Usage:   "macscope proc [--json] <pid-or-name>",
			Summary: "Triage a running process by PID or name.",
			Run:     runProc,
		},
		{
			Name:    "attach",
			Usage:   "macscope attach [--json] [--last 30m] <pid>",
			Summary: "Explain likely LLDB attach failures using signing, group, and log evidence.",
			Run:     runAttach,
		},
		{
			Name:    "persist",
			Usage:   "macscope persist [--json] [--dir <launchd-dir>]",
			Summary: "Inspect launchd persistence locations and explain suspicious launch items.",
			Run:     runPersist,
		},
		{
			Name:    "tcc",
			Usage:   "macscope tcc [--json] [--last 30m] | --watch",
			Summary: "Inspect recent or live TCC/privacy denial logs.",
			Run:     runTCC,
		},
		{
			Name:    "es",
			Usage:   "macscope es [--json] [--last 30m] | --watch",
			Summary: "Inspect EndpointSecurity entitlement and access-denial logs.",
			Run:     runES,
		},
		{
			Name:    "vpn",
			Usage:   "macscope vpn [--json] [--last 60m] [vpn-name]",
			Summary: "Inspect VPN service state, utun interfaces, DNS, proxy, routes, and recent logs.",
			Run:     runVPN,
		},
		{
			Name:    "panic",
			Usage:   "macscope panic --last | --file <panic-file> | --since 48h [--json]",
			Summary: "Parse panic logs and classify watchdog/kernel reboot evidence.",
			Run:     runPanic,
		},
		{
			Name:    "timeline",
			Usage:   "macscope timeline --pid <pid> [--last 30m] [--json|--jsonl]",
			Summary: "Correlate process, log, policy, and system events into one timeline.",
			Run:     runTimeline,
		},
		{
			Name:    "completion",
			Usage:   "macscope completion <bash|zsh|fish>",
			Summary: "Generate shell completion scripts.",
			Run:     runCompletion,
		},
	}
}

func printHelp(w io.Writer) {
	printUsage(w)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	for _, cmd := range commands() {
		fmt.Fprintf(w, "  %s\n", cmd.Usage)
		fmt.Fprintf(w, "      %s\n", cmd.Summary)
	}
	fmt.Fprintln(w)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "macscope - modern macOS introspection and triage toolkit")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope <command> [flags]")
	fmt.Fprintln(w, "  macscope help")
	fmt.Fprintln(w, "  macscope version")
}
