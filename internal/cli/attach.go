package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	attachreport "github.com/jdefrancesco/macscope/internal/attach"
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/process"
)

type attachFlags struct {
	json bool
	help bool
	last string
	pid  int
}

func runAttach(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseAttachFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printAttachHelp(streams.Out)
		return nil
	}

	report, err := attachreport.Analyze(ctx, flags.pid, flags.last, collect.Runner{})
	if err != nil {
		return err
	}
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderAttachReport(streams.Out, report)
}

func parseAttachFlags(args []string) (attachFlags, error) {
	flags := attachFlags{last: "30m"}
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
				return attachFlags{}, errors.New("--last requires a duration like 30m")
			}
			flags.last = args[i]
		default:
			if flags.pid != 0 {
				return attachFlags{}, errors.New("usage: macscope attach [--json] [--last 30m] <pid>")
			}
			pid, ok := process.ParsePID(arg)
			if !ok {
				return attachFlags{}, fmt.Errorf("attach requires a numeric pid, got %q", arg)
			}
			flags.pid = pid
		}
	}
	if flags.help {
		return flags, nil
	}
	if flags.pid == 0 {
		return attachFlags{}, errors.New("usage: macscope attach [--json] [--last 30m] <pid>")
	}
	return flags, nil
}

func printAttachHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope attach [--json] [--last 30m] <pid>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Explain likely LLDB attach failures using process identity, developer group membership, signing state, and recent policy logs.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Native tools:")
	fmt.Fprintln(w, "  ps, dseditgroup, codesign, log")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json       Emit stable JSON.")
	fmt.Fprintln(w, "  --last 30m   Log window for attach-relevant unified logging.")
}

func renderAttachReport(w io.Writer, report attachreport.Report) error {
	tw := output.NewTextWriter(w)

	if err := tw.Section("Target Process"); err != nil {
		return err
	}
	for _, kv := range [][2]string{
		{"PID", strconv.Itoa(report.Process.PID)},
		{"Name", fallback(report.Process.Name, "unknown")},
		{"User", fallback(report.Process.User, "unknown")},
		{"Path", fallback(report.Process.Path, "unknown")},
		{"Command", fallback(report.Process.Command, "unknown")},
	} {
		if err := tw.KeyValue(kv[0], kv[1]); err != nil {
			return err
		}
	}

	if err := tw.Section("Debugger Prerequisites"); err != nil {
		return err
	}
	if err := tw.KeyValue("Current User", fallback(report.CurrentUser, "unknown")); err != nil {
		return err
	}
	if err := tw.KeyValue("_developer", groupStatus(report.DeveloperGroup)); err != nil {
		return err
	}

	if err := tw.Section("Signature / Policy"); err != nil {
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

	if err := tw.Section("Recent Attach Logs"); err != nil {
		return err
	}
	if len(report.RecentLogLines) == 0 {
		if err := tw.Bullet("no attach-relevant log lines returned for the selected window"); err != nil {
			return err
		}
	} else {
		for _, line := range report.RecentLogLines {
			if err := tw.Bullet(line); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Findings"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		if err := tw.Bullet("no specific attach blockers classified"); err != nil {
			return err
		}
	} else {
		for _, finding := range report.Findings {
			if err := renderFindingWithEvidence(tw, finding.Category, finding.Severity, finding.Confidence, finding.Source, finding.Evidence); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Interpretation Checklist"); err != nil {
		return err
	}
	for _, hint := range report.InterpretationHints {
		if err := tw.Bullet(hint); err != nil {
			return err
		}
	}
	return nil
}

func groupStatus(check attachreport.GroupCheck) string {
	if check.Message == "" {
		return strconv.FormatBool(check.Member)
	}
	return fmt.Sprintf("%t - %s", check.Member, check.Message)
}
