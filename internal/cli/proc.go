package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/proctriage"
)

type procFlags struct {
	json  bool
	help  bool
	query string
}

func runProc(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseProcFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printProcHelp(streams.Out)
		return nil
	}

	report, err := proctriage.Analyze(ctx, flags.query, collect.Runner{})
	if err != nil {
		return err
	}
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderProcReport(streams.Out, report)
}

func parseProcFlags(args []string) (procFlags, error) {
	var flags procFlags
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		default:
			if strings.HasPrefix(arg, "-") {
				return procFlags{}, fmt.Errorf("unknown proc flag: %s", arg)
			}
			if flags.query != "" {
				return procFlags{}, errors.New("usage: macscope proc [--json] <pid-or-name>")
			}
			flags.query = arg
		}
	}
	if flags.help {
		return flags, nil
	}
	if flags.query == "" {
		return procFlags{}, errors.New("usage: macscope proc [--json] <pid-or-name>")
	}
	return flags, nil
}

func printProcHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope proc [--json] <pid-or-name>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Resolve a process by PID or name, show identity details, and summarize signing state.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json   Emit stable JSON.")
}

func renderProcReport(w io.Writer, report proctriage.Report) error {
	tw := output.NewTextWriter(w)

	if err := tw.Section("Process"); err != nil {
		return err
	}
	for _, kv := range [][2]string{
		{"PID", fmt.Sprintf("%d", report.Process.PID)},
		{"PPID", fmt.Sprintf("%d", report.Process.PPID)},
		{"Name", fallback(report.Process.Name, "unknown")},
		{"User", fallback(report.Process.User, "unknown")},
		{"Group", fallback(report.Process.Group, "unknown")},
		{"State", fallback(report.Process.Stat, "unknown")},
		{"Path", fallback(report.Process.Path, "unknown")},
		{"Command", fallback(report.Process.Command, "unknown")},
	} {
		if err := tw.KeyValue(kv[0], kv[1]); err != nil {
			return err
		}
	}

	if err := tw.Section("Signature"); err != nil {
		return err
	}
	if report.Process.Path == "" {
		if err := tw.Bullet("executable path unavailable; signing was not checked"); err != nil {
			return err
		}
	} else {
		if err := tw.KeyValue("Identifier", fallback(report.Signing.Details.Identifier, "unknown")); err != nil {
			return err
		}
		if err := tw.KeyValue("Team ID", fallback(report.Signing.Details.TeamIdentifier, "unknown")); err != nil {
			return err
		}
		if err := tw.KeyValue("Verify", signingStatus(report.Signing.Verification.Valid, report.Signing.Verification.Message)); err != nil {
			return err
		}
	}

	if err := tw.Section("Findings"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		if err := tw.Bullet("no notable process signing findings"); err != nil {
			return err
		}
	} else {
		for _, finding := range report.Findings {
			if err := tw.Bullet(fmt.Sprintf("%s severity=%s confidence=%.2f source=%s", finding.Category, finding.Severity, finding.Confidence, finding.Source)); err != nil {
				return err
			}
			for _, evidence := range finding.Evidence {
				if err := tw.Bullet("evidence: " + evidence); err != nil {
					return err
				}
			}
		}
	}

	if err := tw.Section("Next Steps"); err != nil {
		return err
	}
	for _, step := range report.NextSteps {
		if err := tw.Bullet(step); err != nil {
			return err
		}
	}
	return nil
}

func signingStatus(valid bool, message string) string {
	if valid {
		if message == "" {
			return "valid"
		}
		return "valid - " + message
	}
	if message == "" {
		return "invalid or unavailable"
	}
	return "invalid - " + message
}
