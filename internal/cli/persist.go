package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/jdefrancesco/macscope/internal/launchd"
	"github.com/jdefrancesco/macscope/internal/output"
)

type persistFlags struct {
	json bool
	help bool
	dirs []string
}

func runPersist(ctx context.Context, args []string, streams output.Streams) error {
	_ = ctx

	flags, err := parsePersistFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printPersistHelp(streams.Out)
		return nil
	}

	report := launchd.AnalyzeDirs(flags.dirs)
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderPersistReport(streams.Out, report)
}

func parsePersistFlags(args []string) (persistFlags, error) {
	var flags persistFlags
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		case "--dir":
			i++
			if i >= len(args) {
				return persistFlags{}, errors.New("--dir requires a launchd directory path")
			}
			flags.dirs = append(flags.dirs, args[i])
		default:
			return persistFlags{}, fmt.Errorf("unknown persist arg: %s", arg)
		}
	}
	return flags, nil
}

func printPersistHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope persist [--json] [--dir <launchd-dir>]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Inspect launchd persistence plists and explain suspicious launch items.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Default directories:")
	fmt.Fprintln(w, "  /Library/LaunchAgents")
	fmt.Fprintln(w, "  /Library/LaunchDaemons")
	fmt.Fprintln(w, "  ~/Library/LaunchAgents")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json        Emit stable JSON.")
	fmt.Fprintln(w, "  --dir <path>  Inspect one additional or replacement launchd directory. Repeatable.")
}

func renderPersistReport(w io.Writer, report launchd.Report) error {
	tw := output.NewTextWriter(w)

	if err := tw.Section("Launchd Persistence"); err != nil {
		return err
	}
	if err := tw.KeyValue("Directories", fmt.Sprintf("%d", len(report.Directories))); err != nil {
		return err
	}
	if err := tw.KeyValue("Jobs Parsed", fmt.Sprintf("%d", len(report.Jobs))); err != nil {
		return err
	}
	if err := tw.KeyValue("Findings", fmt.Sprintf("%d", len(report.Findings))); err != nil {
		return err
	}

	if err := tw.Section("Directories"); err != nil {
		return err
	}
	for _, dir := range report.Directories {
		if err := tw.Bullet(dir); err != nil {
			return err
		}
	}

	if err := tw.Section("Findings"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		if err := tw.Bullet("no suspicious launchd persistence findings"); err != nil {
			return err
		}
	} else {
		for _, finding := range report.Findings {
			label := fallback(finding.JobLabel, filepath.Base(finding.JobPath))
			details := []string{
				fmt.Sprintf("label: %s", label),
				fmt.Sprintf("score: %d", finding.Score),
			}
			if finding.Program != "" {
				details = append(details, "program: "+finding.Program)
			}
			details = append(details, "plist: "+finding.JobPath)
			for _, evidence := range finding.Evidence {
				details = append(details, "evidence: "+evidence)
			}
			if err := renderFinding(tw, finding.Category, finding.Severity, finding.Confidence, "", details); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Jobs"); err != nil {
		return err
	}
	if len(report.Jobs) == 0 {
		if err := tw.Bullet("none parsed"); err != nil {
			return err
		}
	} else {
		for _, job := range report.Jobs {
			label := fallback(job.Label, filepath.Base(job.Path))
			detail := label
			if program := launchd.EffectiveProgram(job); program != "" {
				detail += " -> " + program
			}
			if job.RunAtLoad {
				detail += " RunAtLoad"
			}
			if job.KeepAlive {
				detail += " KeepAlive"
			}
			if err := tw.Bullet(detail); err != nil {
				return err
			}
		}
	}

	if len(report.Errors) > 0 {
		if err := tw.Section("Parse Errors"); err != nil {
			return err
		}
		for _, parseErr := range report.Errors {
			if err := tw.Bullet(parseErr); err != nil {
				return err
			}
		}
	}

	return nil
}
