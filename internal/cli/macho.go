package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	machoreport "github.com/jdefrancesco/macscope/internal/macho"
	"github.com/jdefrancesco/macscope/internal/output"
)

type machoFlags struct {
	json   bool
	full   bool
	help   bool
	triage bool
	path   string
}

func runMacho(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseMachoFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printMachoHelp(streams.Out)
		return nil
	}

	report, err := machoreport.Analyze(ctx, flags.path, machoreport.Options{
		Full: flags.full,
	})
	if err != nil {
		return err
	}

	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	if flags.triage {
		return renderMachoTriageReport(streams.Out, report)
	}
	return renderMachoReport(streams.Out, report)
}

func parseMachoFlags(args []string) (machoFlags, error) {
	var flags machoFlags

	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		case "--full":
			flags.full = true
		case "--triage":
			flags.triage = true
		default:
			if strings.HasPrefix(arg, "-") {
				return machoFlags{}, fmt.Errorf("unknown macho flag: %s", arg)
			}
			if flags.path != "" {
				return machoFlags{}, errors.New("usage: macscope macho [--json] [--full] [--triage] <path>")
			}
			flags.path = arg
		}
	}

	if flags.help {
		return flags, nil
	}
	if flags.path == "" {
		return machoFlags{}, errors.New("usage: macscope macho [--json] [--full] [--triage] <path>")
	}

	return flags, nil
}

func printMachoHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope macho [--json] [--full] [--triage] <path>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Inspect a Mach-O binary or .app bundle.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "The command reads file identity directly and wraps these native macOS tools:")
	fmt.Fprintln(w, "  file, lipo, codesign, spctl, xattr, otool")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json     Emit stable JSON.")
	fmt.Fprintln(w, "  --full     Include raw command output in JSON.")
	fmt.Fprintln(w, "  --triage   Show a compact file-specific breakdown with an overall triage score.")
}

func renderMachoReport(w io.Writer, report machoreport.Report) error {
	tw := output.NewTextWriter(w)

	if err := tw.Section("Target"); err != nil {
		return err
	}
	for _, kv := range [][2]string{
		{"Input", report.InputPath},
		{"Binary", report.BinaryPath},
		{"Size", fmt.Sprintf("%d bytes", report.SizeBytes)},
		{"SHA-256", report.SHA256},
	} {
		if err := tw.KeyValue(kv[0], kv[1]); err != nil {
			return err
		}
	}

	if err := tw.Section("Identity"); err != nil {
		return err
	}
	if err := tw.KeyValue("Type", fallback(report.FileType, "unknown")); err != nil {
		return err
	}
	if err := tw.KeyValue("Architectures", joinOr(report.Architectures, "unknown")); err != nil {
		return err
	}

	if err := tw.Section("Signature / Policy"); err != nil {
		return err
	}
	if err := tw.KeyValue("Identifier", fallback(report.CodeSignature.Identifier, "unknown")); err != nil {
		return err
	}
	if err := tw.KeyValue("Team ID", fallback(report.CodeSignature.TeamIdentifier, "unknown")); err != nil {
		return err
	}
	if err := tw.KeyValue("Platform ID", fallback(report.CodeSignature.PlatformIdentifier, "none")); err != nil {
		return err
	}
	if err := tw.KeyValue("Authorities", joinOr(report.CodeSignature.Authorities, "unknown")); err != nil {
		return err
	}
	if err := tw.KeyValue("codesign verify", verificationStatus(report)); err != nil {
		return err
	}
	if err := tw.KeyValue("Gatekeeper", gatekeeperStatus(report)); err != nil {
		return err
	}

	if err := tw.Section("Extended Attributes"); err != nil {
		return err
	}
	if len(report.ExtendedAttributes) == 0 {
		if err := tw.Bullet("none reported"); err != nil {
			return err
		}
	} else {
		for _, attr := range report.ExtendedAttributes {
			value := attr.Value
			if value != "" {
				value = " = " + value
			}
			if err := tw.Bullet(attr.Name + value); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Linked Libraries"); err != nil {
		return err
	}
	if len(report.LinkedLibraries) == 0 {
		if err := tw.Bullet("none reported"); err != nil {
			return err
		}
	} else {
		for _, lib := range report.LinkedLibraries {
			if err := tw.Bullet(lib); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Findings"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		return tw.Bullet("no notable signing, Gatekeeper, or quarantine findings")
	}
	for _, finding := range report.Findings {
		line := fmt.Sprintf("%s severity=%s confidence=%.2f source=%s", finding.Category, finding.Severity, finding.Confidence, finding.Source)
		if err := tw.Bullet(line); err != nil {
			return err
		}
		for _, evidence := range finding.Evidence {
			if err := tw.Bullet("evidence: " + evidence); err != nil {
				return err
			}
		}
	}

	return nil
}

func renderMachoTriageReport(w io.Writer, report machoreport.Report) error {
	tw := output.NewTextWriter(w)
	triage := report.Triage

	if err := tw.Section("File Triage"); err != nil {
		return err
	}
	if err := tw.KeyValue("Score", fmt.Sprintf("%d/100 %s", triage.Score, output.Level(triage.Level))); err != nil {
		return err
	}
	if err := tw.KeyValue("Summary", triage.Summary); err != nil {
		return err
	}

	if err := tw.Section("File Specifics"); err != nil {
		return err
	}
	for _, kv := range [][2]string{
		{"Input", report.InputPath},
		{"Binary", report.BinaryPath},
		{"Size", fmt.Sprintf("%d bytes", report.SizeBytes)},
		{"SHA-256", report.SHA256},
		{"Type", fallback(report.FileType, "unknown")},
		{"Architectures", joinOr(report.Architectures, "unknown")},
		{"Identifier", fallback(report.CodeSignature.Identifier, "unknown")},
		{"Team ID", fallback(report.CodeSignature.TeamIdentifier, "unknown")},
		{"codesign verify", verificationStatus(report)},
		{"Gatekeeper", gatekeeperStatus(report)},
		{"Linked Libraries", fmt.Sprintf("%d", len(report.LinkedLibraries))},
		{"Extended Attributes", fmt.Sprintf("%d", len(report.ExtendedAttributes))},
	} {
		if err := tw.KeyValue(kv[0], kv[1]); err != nil {
			return err
		}
	}

	if err := tw.Section("Triage Signals"); err != nil {
		return err
	}
	if len(triage.Signals) == 0 {
		if err := tw.Bullet("no notable static triage signals"); err != nil {
			return err
		}
	} else {
		for _, signal := range triage.Signals {
			if err := tw.Bullet(fmt.Sprintf("%s +%d - %s", signal.Category, signal.Points, signal.Evidence)); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Recommended Actions"); err != nil {
		return err
	}
	for _, action := range triage.RecommendedActions {
		if err := tw.Bullet(action); err != nil {
			return err
		}
	}

	return nil
}

func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}

func joinOr(values []string, replacement string) string {
	if len(values) == 0 {
		return replacement
	}
	return strings.Join(values, ", ")
}

func verificationStatus(report machoreport.Report) string {
	verify := report.CodeSignatureVerify
	if verify.Valid {
		if verify.Message == "" {
			return "valid"
		}
		return "valid - " + verify.Message
	}
	if report.CodeSignature.PlatformIdentifier != "" && strings.Contains(strings.ToLower(verify.Raw), "cssmerr_tp_not_trusted") {
		return "trust-policy warning - Apple platform signature metadata present, but local trust check returned " + verify.Message
	}
	if verify.Message == "" {
		return "invalid or unavailable"
	}
	return "invalid - " + verify.Message
}

func gatekeeperStatus(report machoreport.Report) string {
	assessment := report.GatekeeperAssessment
	if assessment.Raw == "" {
		return "unknown"
	}
	if report.CodeSignature.PlatformIdentifier != "" && strings.Contains(strings.ToLower(assessment.Raw), "internal error in code signing subsystem") {
		return "assessment unavailable - Apple platform signature metadata present"
	}
	if assessment.Accepted {
		if assessment.Source == "" {
			return "accepted"
		}
		return "accepted - source=" + assessment.Source
	}
	if assessment.Source == "" {
		return "rejected"
	}
	return "rejected - source=" + assessment.Source
}
