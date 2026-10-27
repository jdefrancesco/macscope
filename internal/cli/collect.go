package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jdefrancesco/macscope/internal/evidence"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/process"
)

const collectUsage = "macscope collect --pid <pid> --out <new-directory> [--reports proc,ipc,maps,tree] [--json]"

type collectFlags struct {
	pid        int
	out        string
	reports    []string
	json, help bool
}

func parseCollectFlags(args []string) (collectFlags, error) {
	var flags collectFlags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			flags.help = true
		case "--json":
			flags.json = true
		case "--pid", "--out", "--reports":
			flag := args[i]
			i++
			if i >= len(args) || args[i] == "" {
				return collectFlags{}, fmt.Errorf("%s requires a value", flag)
			}
			switch flag {
			case "--pid":
				pid, ok := process.ParsePID(args[i])
				if !ok {
					return collectFlags{}, errors.New("--pid requires a positive PID")
				}
				flags.pid = pid
			case "--out":
				flags.out = args[i]
			case "--reports":
				flags.reports = strings.Split(args[i], ",")
				for j := range flags.reports {
					flags.reports[j] = strings.TrimSpace(flags.reports[j])
				}
				if err := evidence.ValidateReports(flags.reports); err != nil {
					return collectFlags{}, err
				}
			}
		default:
			return collectFlags{}, fmt.Errorf("unknown collect argument: %s", args[i])
		}
	}
	if !flags.help && (flags.pid == 0 || flags.out == "") {
		return collectFlags{}, errors.New("usage: " + collectUsage)
	}
	return flags, nil
}

func runCollect(ctx context.Context, args []string, streams output.Streams) error {
	return runCollectWithRunner(ctx, args, streams, nil)
}

func runCollectWithRunner(ctx context.Context, args []string, streams output.Streams, runner process.CommandRunner) error {
	flags, err := parseCollectFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printCollectHelp(streams.Out)
		return nil
	}
	if _, err := fmt.Fprintln(streams.Err, "Collecting raw evidence locally; files may contain paths, arguments, and identifiers. Nothing is uploaded."); err != nil {
		return err
	}
	version := currentVersionInfo()
	manifest, collectErr := evidence.Create(ctx, evidence.Options{PID: flags.pid, Out: flags.out, Reports: flags.reports, Version: version.Version, Commit: version.Commit, Runner: runner})
	if manifest.Directory == "" {
		return collectErr
	}
	if flags.json {
		err = output.WriteJSON(streams.Out, manifest)
	} else {
		err = renderBundle(streams.Out, manifest)
	}
	if err != nil {
		return err
	}
	return collectErr
}

func printCollectHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:\n  "+collectUsage)
	fmt.Fprintln(w, "\nSave selected process reports and raw command stdout/stderr locally.")
	fmt.Fprintln(w, "  --pid       Exact positive target PID.")
	fmt.Fprintln(w, "  --out       New directory whose parent exists; existing paths are never overwritten.")
	fmt.Fprintln(w, "  --reports   Any unique selection of proc,ipc,maps,tree (default: all four).")
	fmt.Fprintln(w, "  --json      Emit the manifest as JSON; evidence remains in the directory.")
	fmt.Fprintln(w, "\nRequires macOS. Uses ps, codesign, lsmp, lsof, and vmmap for the selected reports.")
	fmt.Fprintln(w, "Writes a private directory (0700), JSON files (0600), commands.json, and manifest.json.")
	fmt.Fprintln(w, "The manifest records UTC capture times, source version, report statuses, and SHA-256 hashes")
	fmt.Fprintln(w, "of all evidence files except the manifest itself. Hashes detect changes, not authenticity.")
	fmt.Fprintln(w, "Raw evidence can contain sensitive paths, arguments, entitlements, and identifiers.")
	fmt.Fprintln(w, "Collection is read-only apart from the explicit local bundle; no sudo or network submission.")
	fmt.Fprintln(w, "Permission denials are retained. Snapshots are sequential, not atomic; partial bundles are preserved.")
}

func renderBundle(w io.Writer, manifest evidence.Manifest) error {
	tw := output.NewTextWriter(w)
	if err := tw.Section("Local Evidence Bundle"); err != nil {
		return err
	}
	for _, kv := range [][2]string{{"Directory", manifest.Directory}, {"Status", manifest.Status}, {"Manifest", fallback(manifest.ManifestPath, "not written; partial files retained")}, {"Evidence files", fmt.Sprint(len(manifest.Files))}} {
		if err := tw.KeyValue(kv[0], ipcText(kv[1], false)); err != nil {
			return err
		}
	}
	for _, report := range manifest.Reports {
		if err := tw.Bullet(report.Name + ": " + report.Status); err != nil {
			return err
		}
	}
	for _, warning := range manifest.Warnings {
		if err := tw.Bullet(ipcText(warning, false)); err != nil {
			return err
		}
	}
	return nil
}
