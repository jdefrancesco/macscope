package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/process"
	"github.com/jdefrancesco/macscope/internal/timeline"
)

type timelineFlags struct {
	json  bool
	jsonl bool
	help  bool
	last  string
	pid   int
}

func runTimeline(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseTimelineFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printTimelineHelp(streams.Out)
		return nil
	}

	report := timeline.Analyze(ctx, flags.pid, flags.last, collect.Runner{})
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	if flags.jsonl {
		return writeTimelineJSONL(streams.Out, report)
	}
	return renderTimelineReport(streams.Out, report)
}

func parseTimelineFlags(args []string) (timelineFlags, error) {
	flags := timelineFlags{last: "30m"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		case "--jsonl":
			flags.jsonl = true
		case "--last":
			i++
			if i >= len(args) {
				return timelineFlags{}, errors.New("--last requires a duration like 30m")
			}
			flags.last = args[i]
		case "--pid":
			i++
			if i >= len(args) {
				return timelineFlags{}, errors.New("--pid requires a numeric pid")
			}
			pid, ok := process.ParsePID(args[i])
			if !ok {
				return timelineFlags{}, fmt.Errorf("--pid requires a numeric pid, got %q", args[i])
			}
			flags.pid = pid
		default:
			return timelineFlags{}, fmt.Errorf("unknown timeline arg: %s", arg)
		}
	}
	if flags.help {
		return flags, nil
	}
	if flags.json && flags.jsonl {
		return timelineFlags{}, errors.New("choose only one output format: --json or --jsonl")
	}
	if flags.pid == 0 {
		return timelineFlags{}, errors.New("usage: macscope timeline --pid <pid> [--last 30m] [--json|--jsonl]")
	}
	return flags, nil
}

func printTimelineHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope timeline --pid <pid> [--last 30m] [--json|--jsonl]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Correlate process identity, signing state, and attach/policy log events into one normalized timeline.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Native tools:")
	fmt.Fprintln(w, "  ps, codesign, log")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --pid <pid>   Target process ID.")
	fmt.Fprintln(w, "  --last 30m    Unified-log window.")
	fmt.Fprintln(w, "  --json        Emit the full report as JSON.")
	fmt.Fprintln(w, "  --jsonl       Emit normalized events as JSON Lines.")
}

func renderTimelineReport(w io.Writer, report timeline.Report) error {
	tw := output.NewTextWriter(w)
	if err := tw.Section("Timeline"); err != nil {
		return err
	}
	if err := tw.KeyValue("PID", fmt.Sprintf("%d", report.PID)); err != nil {
		return err
	}
	if err := tw.KeyValue("Window", fallback(report.Window, "30m")); err != nil {
		return err
	}
	if err := tw.KeyValue("Events", fmt.Sprintf("%d", len(report.Events))); err != nil {
		return err
	}
	if err := tw.KeyValue("Findings", fmt.Sprintf("%d", len(report.Findings))); err != nil {
		return err
	}

	if err := tw.Section("Process"); err != nil {
		return err
	}
	if report.Process.PID == 0 {
		if err := tw.Bullet("process identity unavailable"); err != nil {
			return err
		}
	} else {
		for _, kv := range [][2]string{
			{"Name", fallback(report.Process.Name, "unknown")},
			{"User", fallback(report.Process.User, "unknown")},
			{"Path", fallback(report.Process.Path, "unknown")},
			{"Command", fallback(report.Process.Command, "unknown")},
		} {
			if err := tw.KeyValue(kv[0], kv[1]); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Findings"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		if err := tw.Bullet("no correlated policy or signing findings"); err != nil {
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

	if err := tw.Section("Events"); err != nil {
		return err
	}
	if len(report.Events) == 0 {
		if err := tw.Bullet("none collected"); err != nil {
			return err
		}
	} else {
		for _, event := range report.Events {
			parts := []string{}
			if event.Time != "" {
				parts = append(parts, event.Time)
			}
			parts = append(parts, event.Source, event.Category, output.Level(strings.ToUpper(event.Severity)))
			parts = append(parts, event.Message)
			if err := tw.Bullet(strings.Join(parts, " | ")); err != nil {
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

func writeTimelineJSONL(w io.Writer, report timeline.Report) error {
	encoder := json.NewEncoder(w)
	for _, event := range report.Events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}
