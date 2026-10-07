package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/jdefrancesco/macscope/internal/diskspace"
	"github.com/jdefrancesco/macscope/internal/output"
)

type diskFlags struct {
	json bool
	all  bool
	full bool
	help bool
	path string
}

func runDisk(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseDiskFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printDiskHelp(streams.Out)
		return nil
	}
	report, err := diskspace.Analyze(ctx, diskspace.Options{Path: flags.path, All: flags.all})
	if err != nil {
		return err
	}
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderDiskReport(streams.Out, report, flags.full)
}

func parseDiskFlags(args []string) (diskFlags, error) {
	var flags diskFlags
	positional := false
	for _, arg := range args {
		if !positional {
			switch arg {
			case "--":
				positional = true
				continue
			case "-h", "--help":
				flags.help = true
				continue
			case "--json":
				flags.json = true
				continue
			case "--all":
				flags.all = true
				continue
			case "--full":
				flags.full = true
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return diskFlags{}, fmt.Errorf("unknown disk flag: %s", arg)
			}
		}
		if flags.path != "" || arg == "" {
			return diskFlags{}, errors.New("usage: macscope disk [--json] [--all] [--full] [path]")
		}
		flags.path = arg
	}
	return flags, nil
}

func printDiskHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope disk [--json] [--all] [--full] [path]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "List mounted local disks with size, used space, free space, and capacity percentage.")
	fmt.Fprintln(w, "Supply a file or directory to inspect only the volume containing that path.")
	fmt.Fprintln(w, "Read-only; requires macOS. No sudo is needed; the supplied path must be accessible.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json   Emit stable JSON with sizes in bytes and unredacted paths.")
	fmt.Fprintln(w, "  --all    Include auxiliary system volumes and virtual local mounts.")
	fmt.Fprintln(w, "  --full   Preserve usernames in paths in human output.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Native tool: df -k -P -I (-l when no path is supplied).")
	fmt.Fprintln(w, "APFS volumes can share capacity and free space; do not add their rows together.")
	fmt.Fprintln(w, "Use% is the capacity percentage reported by df, not used / size.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  macscope disk")
	fmt.Fprintln(w, "  macscope disk --json /Volumes/Backup")
	fmt.Fprintln(w, "  macscope disk --all")
}

var diskHomePath = regexp.MustCompile(`/Users/[^/\s]+`)

func renderDiskReport(w io.Writer, report diskspace.Report, full bool) error {
	visiblePath := func(path string) string {
		if full {
			return path
		}
		return diskHomePath.ReplaceAllString(path, "/Users/<redacted>")
	}
	tw := output.NewTextWriter(w)
	if err := tw.Section("Disk Space"); err != nil {
		return err
	}
	if report.RequestedPath != "" {
		if err := tw.KeyValue("Path", visiblePath(report.RequestedPath)); err != nil {
			return err
		}
	}
	if len(report.Volumes) == 0 {
		return tw.Bullet("no visible local disks; use --all to include auxiliary and virtual mounts")
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "  Mount\tFilesystem\tSize\tUsed\tFree\tUse%"); err != nil {
		return err
	}
	for _, volume := range report.Volumes {
		if _, err := fmt.Fprintf(table, "  %s\t%s\t%s\t%s\t%s\t%d%%\n",
			visiblePath(volume.MountPoint), visiblePath(volume.Filesystem),
			formatBytes(volume.TotalBytes), formatBytes(volume.UsedBytes), formatBytes(volume.AvailableBytes), volume.CapacityPercent); err != nil {
			return err
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	return tw.Detail("APFS volumes can share capacity and free space; do not add rows together.")
}
