package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jdefrancesco/macscope/internal/output"
)

const version = "dev"

type commandSpec struct {
	Name      string
	Usage     string
	Summary   string
	Milestone string
	Run       func(context.Context, []string, output.Streams) error
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
	case "version", "--version":
		fmt.Fprintf(streams.Out, "macscope %s\n", version)
		return 0
	}

	cmd, ok := commandByName(args[0])
	if !ok {
		fmt.Fprintf(streams.Err, "unknown command: %s\n\n", args[0])
		printUsage(streams.Err)
		return 2
	}

	if cmd.Run == nil {
		fmt.Fprintf(streams.Err, "%q is recognized, but is not implemented in the Go CLI yet.\n", cmd.Name)
		fmt.Fprintf(streams.Err, "Planned milestone: %s\n", cmd.Milestone)
		fmt.Fprintf(streams.Err, "Current fallback: ./macscope.zsh %s\n", strings.Join(args, " "))
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
			Name:      "macho",
			Usage:     "macscope macho [--json] [--full] <path>",
			Summary:   "Inspect binary/app identity, architecture, signing, Gatekeeper, xattrs, and linked libraries.",
			Milestone: "Milestone 2: macho",
			Run:       runMacho,
		},
		{
			Name:      "proc",
			Usage:     "macscope proc [--json] <pid-or-name>",
			Summary:   "Triage a running process by PID or name.",
			Milestone: "Milestone 4: attach/process triage",
			Run:       runProc,
		},
		{
			Name:      "attach",
			Usage:     "macscope attach [--json] [--last 30m] <pid>",
			Summary:   "Explain likely LLDB attach failures using signing, group, and log evidence.",
			Milestone: "Milestone 4: attach",
			Run:       runAttach,
		},
		{
			Name:      "persist",
			Usage:     "macscope persist [--json] [--dir <launchd-dir>]",
			Summary:   "Inspect launchd persistence locations and explain suspicious launch items.",
			Milestone: "Milestone 5: persist",
			Run:       runPersist,
		},
		{
			Name:      "tcc",
			Usage:     "macscope tcc --last 30m | --watch",
			Summary:   "Inspect recent or live TCC/privacy denial logs.",
			Milestone: "Milestone 6: tcc",
		},
		{
			Name:      "es",
			Usage:     "macscope es --last 30m | --watch",
			Summary:   "Inspect EndpointSecurity entitlement and access-denial logs.",
			Milestone: "Milestone 6: es",
		},
		{
			Name:      "vpn",
			Usage:     "macscope vpn [vpn-name]",
			Summary:   "Inspect VPN service state, utun interfaces, DNS, proxy, routes, and recent logs.",
			Milestone: "Milestone 6: vpn",
		},
		{
			Name:      "panic",
			Usage:     "macscope panic --last | --file <panic-file> | --since 48h [--json]",
			Summary:   "Parse panic logs and classify watchdog/kernel reboot evidence.",
			Milestone: "Milestone 3: panic",
			Run:       runPanic,
		},
		{
			Name:      "timeline",
			Usage:     "macscope timeline --pid <pid>",
			Summary:   "Correlate process, log, policy, and system events into one timeline.",
			Milestone: "Milestone 7: timeline and correlation",
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
	fmt.Fprintln(w, "Status:")
	fmt.Fprintln(w, "  The Go CLI skeleton is active. macho, panic, proc, attach, and persist are implemented.")
	fmt.Fprintln(w, "  Remaining feature commands are routed first, then filled in by milestone.")
	fmt.Fprintln(w, "  The existing ./macscope.zsh remains the proof-of-concept fallback during the port.")
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "macscope - modern macOS introspection and triage toolkit")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope <command> [flags]")
	fmt.Fprintln(w, "  macscope help")
	fmt.Fprintln(w, "  macscope version")
}
