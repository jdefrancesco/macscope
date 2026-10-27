package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/memory"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/process"
	"github.com/jdefrancesco/macscope/internal/proctriage"
)

type procFlags struct {
	json  bool
	help  bool
	query string
	maps  bool
	tree  bool
	full  bool
}

const procUsage = "macscope proc [--json] [--maps] [--tree] [--full] <pid-or-name>"

func runProc(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseProcFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printProcHelp(streams.Out)
		return nil
	}

	runner, err := collect.MacOSRunner()
	if err != nil {
		return err
	}
	report, err := proctriage.AnalyzeWithOptions(ctx, proctriage.Options{Query: flags.query, Maps: flags.maps, Tree: flags.tree, Full: flags.full, Runner: runner})
	if err != nil {
		return err
	}
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderProcReportFull(streams.Out, report, flags.full)
}

func parseProcFlags(args []string) (procFlags, error) {
	var flags procFlags
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		case "--maps":
			flags.maps = true
		case "--tree":
			flags.tree = true
		case "--full":
			flags.full = true
		default:
			if strings.HasPrefix(arg, "-") {
				return procFlags{}, fmt.Errorf("unknown proc flag: %s", arg)
			}
			if flags.query != "" {
				return procFlags{}, errors.New("usage: " + procUsage)
			}
			flags.query = arg
		}
	}
	if flags.help {
		return flags, nil
	}
	if flags.query == "" {
		return procFlags{}, errors.New("usage: " + procUsage)
	}
	return flags, nil
}

func printProcHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  "+procUsage)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Resolve a process by PID or name, show identity details, and summarize signing state.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json   Emit stable JSON.")
	fmt.Fprintln(w, "  --maps   Show vmmap regions and image mappings (addresses, sizes, protections).")
	fmt.Fprintln(w, "  --tree   Show parents and direct children with ps start times and executable paths.")
	fmt.Fprintln(w, "  --full   Preserve home paths and include raw vmmap output.")
	fmt.Fprintln(w, "Requires macOS. Read-only; vmmap may be denied task access, even as root.")
	fmt.Fprintln(w, "Image mappings are not a complete dyld inventory. Region sizes are virtual, not resident RAM.")
	fmt.Fprintln(w, "Ancestry is a snapshot; ps start times have second resolution and use local time.")
}

func renderProcReportFull(w io.Writer, report proctriage.Report, full bool) error {
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
		{"Started (local)", fallback(report.Process.StartedAt, "unknown")},
	} {
		if err := tw.KeyValue(kv[0], ipcText(kv[1], full)); err != nil {
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
		if err := tw.KeyValue("Identifier", ipcText(fallback(report.Signing.Details.Identifier, "unknown"), full)); err != nil {
			return err
		}
		if err := tw.KeyValue("Team ID", ipcText(fallback(report.Signing.Details.TeamIdentifier, "unknown"), full)); err != nil {
			return err
		}
		if err := tw.KeyValue("Verify", ipcText(signingStatus(report.Signing.Verification.Valid, report.Signing.Verification.Message), full)); err != nil {
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
			evidence := make([]string, len(finding.Evidence))
			for i, item := range finding.Evidence {
				evidence[i] = ipcText(item, full)
			}
			if err := renderFindingWithEvidence(tw, finding.Category, finding.Severity, finding.Confidence, finding.Source, evidence); err != nil {
				return err
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
	if report.Tree != nil {
		if err := renderProcessTree(tw, *report.Tree, full); err != nil {
			return err
		}
	}
	if report.Maps != nil {
		if err := renderMemoryReport(tw, *report.Maps, full); err != nil {
			return err
		}
	}
	for _, warning := range report.Warnings {
		if err := tw.Bullet(ipcText(warning, full)); err != nil {
			return err
		}
	}
	return nil
}

func renderProcessTree(tw *output.TextWriter, tree process.Tree, full bool) error {
	if err := tw.Section("Process Ancestry"); err != nil {
		return err
	}
	if err := tw.KeyValue("Status", tree.Status); err != nil {
		return err
	}
	if err := tw.KeyValue("Source", tree.Source); err != nil {
		return err
	}
	for _, group := range []struct {
		label string
		nodes []process.Node
	}{{"Parent", tree.Ancestors}, {"Target", []process.Node{tree.Target}}, {"Child", tree.Children}} {
		for _, node := range group.nodes {
			if err := tw.Bullet(ipcText(fmt.Sprintf("%s pid=%d ppid=%d started=%s path=%s", group.label, node.PID, node.PPID, node.StartedAt, node.Path), full)); err != nil {
				return err
			}
		}
	}
	for _, warning := range tree.Warnings {
		if err := tw.Bullet(ipcText(warning, full)); err != nil {
			return err
		}
	}
	return nil
}

func renderMemoryReport(tw *output.TextWriter, report memory.Report, full bool) error {
	if err := tw.Section("Runtime Memory Maps"); err != nil {
		return err
	}
	if err := tw.KeyValue("Status", report.Status); err != nil {
		return err
	}
	if err := tw.KeyValue("Source", report.Source); err != nil {
		return err
	}
	for _, warning := range report.Warnings {
		if err := tw.Bullet(ipcText(warning, full)); err != nil {
			return err
		}
	}
	for _, image := range report.Images {
		if err := tw.Bullet(ipcText("Image: "+image.Path, full)); err != nil {
			return err
		}
	}
	for _, region := range report.Regions {
		line := fmt.Sprintf("%s-%s  %s  %s/%s  %s  %s  %s", region.Start, region.End, formatBytes(int64(region.SizeBytes)), region.Protection, region.MaxProtection, region.Sharing, region.Type, region.Path)
		if err := tw.Bullet(ipcText(line, full)); err != nil {
			return err
		}
	}
	if len(report.Regions) > 0 {
		if err := tw.Bullet("Sizes are virtual address spans, not resident RAM; image mappings reflect the regions reported by vmmap."); err != nil {
			return err
		}
	}
	if full && report.Raw != nil {
		if err := tw.Detail("Raw stdout: " + ipcText(report.Raw.Stdout, true)); err != nil {
			return err
		}
		return tw.Detail("Raw stderr: " + ipcText(report.Raw.Stderr, true))
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
