package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/vpn"
)

type vpnFlags struct {
	json bool
	help bool
	last string
	name string
}

func runVPN(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseVPNFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printVPNHelp(streams.Out)
		return nil
	}

	report := vpn.Analyze(ctx, flags.name, flags.last, collect.Runner{})
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderVPNReport(streams.Out, report)
}

func parseVPNFlags(args []string) (vpnFlags, error) {
	flags := vpnFlags{last: "60m"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		case "--last":
			i++
			if i >= len(args) {
				return vpnFlags{}, errors.New("--last requires a duration like 60m")
			}
			flags.last = args[i]
		default:
			if strings.HasPrefix(arg, "-") {
				return vpnFlags{}, fmt.Errorf("unknown vpn arg: %s", arg)
			}
			if flags.name != "" {
				return vpnFlags{}, errors.New("usage: macscope vpn [--json] [--last 60m] [vpn-name]")
			}
			flags.name = arg
		}
	}
	return flags, nil
}

func printVPNHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope vpn [--json] [--last 60m] [vpn-name]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Inspect VPN state, utun interfaces, DNS, proxy, routes, recent logs, and sleep/wake correlation.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Native tools:")
	fmt.Fprintln(w, "  scutil, ifconfig, route, netstat, log, pmset")
}

func renderVPNReport(w io.Writer, report vpn.Report) error {
	tw := output.NewTextWriter(w)
	if err := tw.Section("VPN Triage"); err != nil {
		return err
	}
	if err := tw.KeyValue("Requested VPN", fallback(report.RequestedName, "none")); err != nil {
		return err
	}
	if err := tw.KeyValue("Log Window", fallback(report.LogWindow, "60m")); err != nil {
		return err
	}
	if err := tw.KeyValue("Services", fmt.Sprintf("%d", len(report.Services))); err != nil {
		return err
	}
	if err := tw.KeyValue("utun Interfaces", fmt.Sprintf("%d", len(report.Interfaces))); err != nil {
		return err
	}
	if err := tw.KeyValue("Findings", fmt.Sprintf("%d", len(report.Findings))); err != nil {
		return err
	}

	if report.SelectedStatus != "" {
		if err := tw.Section("Selected VPN Status"); err != nil {
			return err
		}
		for _, line := range firstLines(report.SelectedStatus, 8) {
			if err := tw.Bullet(line); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Configured Services"); err != nil {
		return err
	}
	if len(report.Services) == 0 {
		if err := tw.Bullet("none reported"); err != nil {
			return err
		}
	} else {
		for _, service := range report.Services {
			line := service.Name
			if service.Status != "" {
				line += " status=" + service.Status
			}
			if err := tw.Bullet(line); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("utun Interfaces"); err != nil {
		return err
	}
	if len(report.Interfaces) == 0 {
		if err := tw.Bullet("none parsed"); err != nil {
			return err
		}
	} else {
		for _, iface := range report.Interfaces {
			line := iface.Name
			if iface.Status != "" {
				line += " status=" + iface.Status
			}
			if len(iface.Addresses) > 0 {
				line += " " + strings.Join(iface.Addresses, " ")
			}
			if err := tw.Bullet(line); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Findings"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		if err := tw.Bullet("no specific VPN blockers classified"); err != nil {
			return err
		}
	} else {
		for _, finding := range report.Findings {
			if err := renderFindingWithEvidence(tw, finding.Category, finding.Severity, finding.Confidence, finding.Source, finding.Evidence); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Recent VPN Logs"); err != nil {
		return err
	}
	if len(report.RecentLogLines) == 0 {
		if err := tw.Bullet("none returned for this window"); err != nil {
			return err
		}
	} else {
		for _, line := range report.RecentLogLines {
			if err := tw.Bullet(line); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Sleep/Wake Correlation"); err != nil {
		return err
	}
	if len(report.SleepWakeLines) == 0 {
		if err := tw.Bullet("none returned"); err != nil {
			return err
		}
	} else {
		for _, line := range report.SleepWakeLines {
			if err := tw.Bullet(line); err != nil {
				return err
			}
		}
	}

	if len(report.CollectionErrors) > 0 {
		if err := tw.Section("Collection Errors"); err != nil {
			return err
		}
		for _, collectionErr := range report.CollectionErrors {
			if err := tw.Bullet(collectionErr); err != nil {
				return err
			}
		}
	}

	return nil
}

func firstLines(input string, limit int) []string {
	var lines []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
		if len(lines) >= limit {
			break
		}
	}
	return lines
}
