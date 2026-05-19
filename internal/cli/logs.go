package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/endpointsecurity"
	"github.com/jdefrancesco/macscope/internal/logquery"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/tcc"
)

type logFlags struct {
	json  bool
	help  bool
	watch bool
	last  string
}

func runTCC(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseLogFlags("tcc", args)
	if err != nil {
		return err
	}
	if flags.help {
		printTCCHelp(streams.Out)
		return nil
	}
	if flags.watch {
		if flags.json {
			return errors.New("--json is not supported with --watch")
		}
		return logquery.Stream(ctx, logquery.TCCPredicate, streams.Out, streams.Err)
	}

	result, err := logquery.Show(ctx, flags.last, logquery.TCCPredicate, collect.Runner{})
	if err != nil && result.Stdout == "" && result.Stderr == "" {
		return err
	}
	report := tcc.Parse(flags.last, result.Stdout+"\n"+result.Stderr)
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderTCCReport(streams.Out, report)
}

func runES(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseLogFlags("es", args)
	if err != nil {
		return err
	}
	if flags.help {
		printESHelp(streams.Out)
		return nil
	}
	if flags.watch {
		if flags.json {
			return errors.New("--json is not supported with --watch")
		}
		return logquery.Stream(ctx, logquery.EndpointSecurityPredicate, streams.Out, streams.Err)
	}

	result, err := logquery.Show(ctx, flags.last, logquery.EndpointSecurityPredicate, collect.Runner{})
	if err != nil && result.Stdout == "" && result.Stderr == "" {
		return err
	}
	report := endpointsecurity.Parse(flags.last, result.Stdout+"\n"+result.Stderr)
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderESReport(streams.Out, report)
}

func parseLogFlags(command string, args []string) (logFlags, error) {
	flags := logFlags{last: "30m"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		case "--watch":
			flags.watch = true
		case "--last":
			i++
			if i >= len(args) {
				return logFlags{}, errors.New("--last requires a duration like 30m")
			}
			flags.last = args[i]
		default:
			return logFlags{}, fmt.Errorf("unknown %s arg: %s", command, arg)
		}
	}
	return flags, nil
}

func printTCCHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope tcc [--json] [--last 30m]")
	fmt.Fprintln(w, "  macscope tcc --watch")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Inspect recent or live TCC/privacy denial logs.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Native tool:")
	fmt.Fprintln(w, "  log show / log stream")
}

func printESHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope es [--json] [--last 30m]")
	fmt.Fprintln(w, "  macscope es --watch")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Inspect EndpointSecurity entitlement and /dev/es access logs.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Native tool:")
	fmt.Fprintln(w, "  log show / log stream")
}

func renderTCCReport(w io.Writer, report tcc.Report) error {
	tw := output.NewTextWriter(w)
	if err := tw.Section("TCC Privacy Logs"); err != nil {
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

	if err := tw.Section("Findings"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		if err := tw.Bullet("no TCC denial findings in this window"); err != nil {
			return err
		}
	} else {
		for _, finding := range report.Findings {
			if err := tw.Bullet(fmt.Sprintf("%s severity=%s confidence=%.2f", finding.Category, finding.Severity, finding.Confidence)); err != nil {
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
		return tw.Bullet("none parsed")
	}
	for _, event := range report.Events {
		line := event.Message
		if event.Service != "" {
			line = event.Service + " - " + line
		}
		if err := tw.Bullet(line); err != nil {
			return err
		}
	}
	return nil
}

func renderESReport(w io.Writer, report endpointsecurity.Report) error {
	tw := output.NewTextWriter(w)
	if err := tw.Section("EndpointSecurity Logs"); err != nil {
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

	if err := tw.Section("Findings"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		if err := tw.Bullet("no EndpointSecurity denial findings in this window"); err != nil {
			return err
		}
	} else {
		for _, finding := range report.Findings {
			if err := tw.Bullet(fmt.Sprintf("%s severity=%s confidence=%.2f", finding.Category, finding.Severity, finding.Confidence)); err != nil {
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
		return tw.Bullet("none parsed")
	}
	for _, event := range report.Events {
		if err := tw.Bullet(event.Message); err != nil {
			return err
		}
	}
	return nil
}
