package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/jdefrancesco/macscope/internal/launchd"
	"github.com/jdefrancesco/macscope/internal/output"
)

func runAgents(ctx context.Context, args []string, streams output.Streams) error {
	return runLaunchInventory(ctx, "agents", args, streams)
}
func runDaemons(ctx context.Context, args []string, streams output.Streams) error {
	return runLaunchInventory(ctx, "daemons", args, streams)
}

func runLaunchInventory(ctx context.Context, kind string, args []string, streams output.Streams) error {
	var dirs []string
	var json, full, help bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			json = true
		case "--full":
			full = true
		case "--help", "-h":
			help = true
		case "--dir":
			i++
			if i >= len(args) || strings.TrimSpace(args[i]) == "" || strings.HasPrefix(args[i], "--") {
				return fmt.Errorf("--dir requires a launchd directory path")
			}
			dirs = append(dirs, args[i])
		default:
			return fmt.Errorf("unknown %s arg: %s", kind, args[i])
		}
	}
	if help {
		fmt.Fprintf(streams.Out, "Usage:\n  macscope %s [--json] [--full] [--dir <path>]\n\n", kind)
		fmt.Fprintln(streams.Out, "List installed launchd plists and startup settings. Daemons load at boot; agents load at login.")
		fmt.Fprintln(streams.Out, "Includes Apple system, local, and (for agents) current-user directories.")
		fmt.Fprintln(streams.Out, "--dir replaces default directories; repeat to inspect offline XML fixtures.")
		fmt.Fprintln(streams.Out, "--json emits raw structured fields; --full preserves usernames in human output.")
		fmt.Fprintln(streams.Out, "Read-only; binary plists use plutil. No sudo required for normally readable directories.")
		fmt.Fprintln(streams.Out, "Plist settings do not prove execution; launchctl overrides and other triggers may apply.")
		return nil
	}
	report, err := launchd.List(ctx, kind, dirs)
	if err != nil {
		return err
	}
	if json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderLaunchInventory(streams.Out, report, full)
}

func renderLaunchInventory(w io.Writer, report launchd.Inventory, full bool) error {
	visible := func(s string) string {
		if full {
			return s
		}
		return diskHomePath.ReplaceAllString(s, "/Users/<redacted>")
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintf(tw, "Launch %s: %d installed jobs\n", report.Kind, len(report.Jobs)); err != nil {
		return err
	}
	fmt.Fprintln(tw, "LABEL\tRUN AT LOAD\tKEEP ALIVE\tPLIST DISABLED\tPROGRAM\tPLIST")
	for _, job := range report.Jobs {
		keep := fmt.Sprint(job.KeepAlive)
		if job.KeepAliveDetail == "dictionary" {
			keep = "conditional"
		}
		fmt.Fprintf(tw, "%s\t%t\t%s\t%t\t%s\t%s\n", visible(job.Label), job.RunAtLoad, keep, job.Disabled, visible(launchd.EffectiveProgram(job)), visible(job.Path))
	}
	fmt.Fprintln(tw, "Daemons load at boot; agents load at login. Settings do not confirm execution or launchctl overrides.")
	for _, err := range report.Errors {
		fmt.Fprintln(tw, "Collection error: "+visible(err))
	}
	return tw.Flush()
}
