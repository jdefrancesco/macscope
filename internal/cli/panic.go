package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jdefrancesco/macscope/internal/output"
	paniclog "github.com/jdefrancesco/macscope/internal/panic"
)

type panicFlags struct {
	json  bool
	help  bool
	mode  string
	file  string
	since string
}

type panicReportSet struct {
	Query   string            `json:"query"`
	Reports []paniclog.Report `json:"reports"`
}

func runPanic(ctx context.Context, args []string, streams output.Streams) error {
	_ = ctx

	flags, err := parsePanicFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printPanicHelp(streams.Out)
		return nil
	}

	switch flags.mode {
	case "file":
		report, err := paniclog.ReadReport(flags.file)
		if err != nil {
			return err
		}
		if flags.json {
			return output.WriteJSON(streams.Out, report)
		}
		return renderPanicReports(streams.Out, []paniclog.Report{report})
	case "last":
		file, err := paniclog.LatestFile(paniclog.DefaultSearchDirs)
		if err != nil {
			return err
		}
		report, err := paniclog.ReadReport(file.Path)
		if err != nil {
			return err
		}
		if flags.json {
			return output.WriteJSON(streams.Out, report)
		}
		return renderPanicReports(streams.Out, []paniclog.Report{report})
	case "since":
		duration, err := time.ParseDuration(flags.since)
		if err != nil || duration <= 0 {
			return fmt.Errorf("invalid --since duration %q; use values like 30m or 48h", flags.since)
		}
		files, err := paniclog.FindFilesSince(paniclog.DefaultSearchDirs, time.Now().Add(-duration))
		if err != nil {
			return err
		}
		reports, err := paniclog.ReadReports(files)
		if err != nil {
			return err
		}
		if flags.json {
			return output.WriteJSON(streams.Out, panicReportSet{
				Query:   "since " + flags.since,
				Reports: reports,
			})
		}
		return renderPanicReports(streams.Out, reports)
	default:
		return errors.New("usage: macscope panic --last | --file <panic-file> | --since 48h [--json]")
	}
}

func parsePanicFlags(args []string) (panicFlags, error) {
	var flags panicFlags

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		case "--last":
			if err := setPanicMode(&flags, "last"); err != nil {
				return panicFlags{}, err
			}
		case "--file":
			if err := setPanicMode(&flags, "file"); err != nil {
				return panicFlags{}, err
			}
			i++
			if i >= len(args) {
				return panicFlags{}, errors.New("--file requires a panic report path")
			}
			flags.file = args[i]
		case "--since":
			if err := setPanicMode(&flags, "since"); err != nil {
				return panicFlags{}, err
			}
			i++
			if i >= len(args) {
				return panicFlags{}, errors.New("--since requires a duration like 48h")
			}
			flags.since = args[i]
		default:
			return panicFlags{}, fmt.Errorf("unknown panic arg: %s", arg)
		}
	}

	if flags.help {
		return flags, nil
	}
	if flags.mode == "" {
		return panicFlags{}, errors.New("usage: macscope panic --last | --file <panic-file> | --since 48h [--json]")
	}

	return flags, nil
}

func setPanicMode(flags *panicFlags, mode string) error {
	if flags.mode != "" && flags.mode != mode {
		return errors.New("choose only one panic source: --last, --file, or --since")
	}
	flags.mode = mode
	return nil
}

func printPanicHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope panic --last [--json]")
	fmt.Fprintln(w, "  macscope panic --file <panic-file> [--json]")
	fmt.Fprintln(w, "  macscope panic --since 48h [--json]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Parse macOS panic reports and classify watchdog/kernel reboot evidence.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Sources:")
	fmt.Fprintln(w, "  --last          Parse the newest panic report under /Library/Logs/DiagnosticReports.")
	fmt.Fprintln(w, "  --file <path>   Parse a specific panic report.")
	fmt.Fprintln(w, "  --since <dur>   Parse panic reports modified within a duration such as 30m or 48h.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json          Emit stable JSON.")
}

func renderPanicReports(w io.Writer, reports []paniclog.Report) error {
	if len(reports) == 0 {
		_, err := fmt.Fprintln(w, "No panic reports found.")
		return err
	}

	for i, report := range reports {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if err := renderPanicReport(w, report); err != nil {
			return err
		}
	}
	return nil
}

func renderPanicReport(w io.Writer, report paniclog.Report) error {
	tw := output.NewTextWriter(w)

	if err := tw.Section("Panic Type"); err != nil {
		return err
	}
	if err := tw.KeyValue("Type", report.PanicType); err != nil {
		return err
	}
	if err := tw.KeyValue("Summary", fallback(report.Summary, "no summary available")); err != nil {
		return err
	}
	if err := tw.KeyValue("Evidence", sourceEvidence(report)); err != nil {
		return err
	}

	if err := tw.Section("Metadata"); err != nil {
		return err
	}
	if report.CPU != nil {
		if err := tw.KeyValue("CPU", strconv.Itoa(*report.CPU)); err != nil {
			return err
		}
	}
	if report.WatchdogTimeoutSec != nil {
		if err := tw.KeyValue("Watchdog Timeout", strconv.Itoa(*report.WatchdogTimeoutSec)+" seconds"); err != nil {
			return err
		}
	}
	for _, kv := range [][2]string{
		{"OS Version", report.OSVersion},
		{"Boot Session UUID", report.BootSessionUUID},
		{"Current Phase", report.CurrentPhase},
		{"Caller", report.CallerAddress},
		{"Saved Report", report.SavedReportPath},
	} {
		if kv[1] == "" {
			continue
		}
		if err := tw.KeyValue(kv[0], kv[1]); err != nil {
			return err
		}
	}
	if report.SOCDPresent != nil {
		if err := tw.KeyValue("SOCD", strconv.FormatBool(*report.SOCDPresent)); err != nil {
			return err
		}
	}

	if err := tw.Section("Likely Causes"); err != nil {
		return err
	}
	if len(report.SuspectedCauses) == 0 {
		if err := tw.Bullet("no specific cause classified from this report"); err != nil {
			return err
		}
	} else {
		for _, cause := range report.SuspectedCauses {
			if err := tw.Bullet(fmt.Sprintf("%s confidence=%.2f", cause.Category, cause.Confidence)); err != nil {
				return err
			}
			for _, evidence := range cause.Evidence {
				if err := tw.Bullet("evidence: " + evidence); err != nil {
					return err
				}
			}
		}
	}

	if err := tw.Section("Indicators"); err != nil {
		return err
	}
	if len(report.Indicators) == 0 && len(report.PreOSMarkers) == 0 {
		if err := tw.Bullet("none detected"); err != nil {
			return err
		}
	} else {
		for _, indicator := range report.Indicators {
			if err := tw.Bullet(indicator); err != nil {
				return err
			}
		}
		for _, marker := range report.PreOSMarkers {
			if err := tw.Bullet("pre-OS marker: " + marker); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Next Checks"); err != nil {
		return err
	}
	for _, check := range nextChecks(report) {
		if err := tw.Bullet(check); err != nil {
			return err
		}
	}

	return nil
}

func sourceEvidence(report paniclog.Report) string {
	if report.SourcePath == "" {
		return "input text"
	}
	return filepath.Base(report.SourcePath)
}

func nextChecks(report paniclog.Report) []string {
	if report.PanicType != paniclog.TypeWatchdogTimeout {
		return []string{"inspect the panic string and kernel extension context"}
	}

	checks := []string{
		"external dock/display state",
		"software update or installer state",
		"third-party system extensions",
	}
	if strings.Contains(strings.Join(report.Indicators, " "), "Thunderbolt") {
		checks = append(checks, "Thunderbolt/USB-C device chain")
	}
	return checks
}
