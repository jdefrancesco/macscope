package cli

import (
	"context"
	"fmt"
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/systemextensions"
	"io"
)

type sysextFlags struct {
	json bool
	help bool
}

func runSysext(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseSysextFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printSysextHelp(streams.Out)
		return nil
	}

	report := systemextensions.Analyze(ctx, collect.Runner{})
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderSysextReport(streams.Out, report)
}

func parseSysextFlags(args []string) (sysextFlags, error) {
	var flags sysextFlags
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		default:
			return sysextFlags{}, fmt.Errorf("unknown sysext arg: %s", arg)
		}
	}
	return flags, nil
}

func printSysextHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope sysext [--json]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Inventory installed system extensions and classify notable states.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Reports network extensions, endpoint security extensions, and driver extensions.")
	fmt.Fprintln(w, "Flags enabled/active state, extensions awaiting user approval, and conflicts")
	fmt.Fprintln(w, "between multiple concurrent network extensions.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Native tools:")
	fmt.Fprintln(w, "  systemextensionsctl")
}

func renderSysextReport(w io.Writer, report systemextensions.Report) error {
	tw := output.NewTextWriter(w)

	byType := map[systemextensions.ExtensionType][]systemextensions.Extension{}
	for _, ext := range report.Extensions {
		byType[ext.Type] = append(byType[ext.Type], ext)
	}

	if err := tw.Section("System Extensions"); err != nil {
		return err
	}
	if err := tw.KeyValue("Total", fmt.Sprintf("%d", len(report.Extensions))); err != nil {
		return err
	}
	if err := tw.KeyValue("Network", fmt.Sprintf("%d", len(byType[systemextensions.TypeNetwork]))); err != nil {
		return err
	}
	if err := tw.KeyValue("Endpoint Security", fmt.Sprintf("%d", len(byType[systemextensions.TypeEndpointSec]))); err != nil {
		return err
	}
	if err := tw.KeyValue("Driver", fmt.Sprintf("%d", len(byType[systemextensions.TypeDriver]))); err != nil {
		return err
	}

	order := []struct {
		t     systemextensions.ExtensionType
		label string
	}{
		{systemextensions.TypeNetwork, "Network Extensions"},
		{systemextensions.TypeEndpointSec, "Endpoint Security Extensions"},
		{systemextensions.TypeDriver, "Driver Extensions"},
		{systemextensions.TypeUnknown, "Other Extensions"},
	}

	for _, entry := range order {
		exts := byType[entry.t]
		if len(exts) == 0 {
			continue
		}
		if err := tw.Section(entry.label); err != nil {
			return err
		}
		for _, ext := range exts {
			line := ext.BundleID
			if ext.Version != "" {
				line += " (" + ext.Version + ")"
			}
			if ext.Name != "" {
				line += "  " + ext.Name
			}
			if err := tw.Bullet(line); err != nil {
				return err
			}
			statusLine := fmt.Sprintf("state=%s  enabled=%s  active=%s  teamID=%s",
				ext.State,
				boolMark(ext.Enabled),
				boolMark(ext.Active),
				ext.TeamID,
			)
			if err := tw.Detail(statusLine); err != nil {
				return err
			}
		}
	}

	if err := tw.Section("Findings"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		if err := tw.Bullet("no notable extension states classified"); err != nil {
			return err
		}
	} else {
		for _, finding := range report.Findings {
			if err := renderFindingWithEvidence(tw, finding.Category, finding.Severity, finding.Confidence, finding.Source, finding.Evidence); err != nil {
				return err
			}
		}
	}

	if len(report.CollectionErrors) > 0 {
		if err := tw.Section("Collection Errors"); err != nil {
			return err
		}
		for _, collErr := range report.CollectionErrors {
			if err := tw.Bullet(collErr); err != nil {
				return err
			}
		}
	}

	return nil
}

func boolMark(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
