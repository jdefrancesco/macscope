package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/jdefrancesco/macscope/internal/ipc"
	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/process"
)

const ipcUsage = "macscope ipc [--json] [--full] <pid-or-name>"

type ipcFlags struct {
	json, full, help bool
	query            string
}

func runIPC(ctx context.Context, args []string, streams output.Streams) error {
	return runIPCWithRunner(ctx, args, streams, nil)
}

func runIPCWithRunner(ctx context.Context, args []string, streams output.Streams, runner process.CommandRunner) error {
	flags, err := parseIPCFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printIPCHelp(streams.Out)
		return nil
	}
	report, err := ipc.Analyze(ctx, ipc.Options{Query: flags.query, Full: flags.full, Runner: runner})
	if err != nil {
		return err
	}
	if flags.json {
		err = output.WriteJSON(streams.Out, report)
	} else {
		err = renderIPCReport(streams.Out, report, flags.full)
	}
	if err != nil {
		return err
	}
	if report.MachPorts.Status == "unavailable" && report.LocalIPC.Status == "unavailable" {
		return errors.New("both IPC sources are unavailable; see source warnings for access or process-exit details")
	}
	return nil
}

func parseIPCFlags(args []string) (ipcFlags, error) {
	var flags ipcFlags
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
			case "--full":
				flags.full = true
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return ipcFlags{}, fmt.Errorf("unknown ipc flag: %s", arg)
			}
		}
		if flags.query != "" || strings.TrimSpace(arg) == "" {
			return ipcFlags{}, errors.New("usage: " + ipcUsage)
		}
		flags.query = arg
	}
	if !flags.help && flags.query == "" {
		return ipcFlags{}, errors.New("usage: " + ipcUsage)
	}
	return flags, nil
}

func printIPCHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:\n  "+ipcUsage)
	fmt.Fprintln(w, "\nShow a process's Mach port rights, queues, and available peer processes using lsmp,")
	fmt.Fprintln(w, "plus Unix sockets, pipes, FIFOs, POSIX shared memory, and semaphores using lsof.")
	fmt.Fprintln(w, "Names use the same newest-match resolution as proc; use a PID for an exact target.")
	fmt.Fprintln(w, "\nFlags:")
	fmt.Fprintln(w, "  --json   Emit stable JSON with source status and unredacted paths.")
	fmt.Fprintln(w, "  --full   Preserve paths and include raw command output (also in JSON).")
	fmt.Fprintln(w, "\nRequires macOS. Read-only; does not invoke sudo or change security policy.")
	fmt.Fprintln(w, "Mach inspection requires task-read access. Root can improve lsmp visibility,")
	fmt.Fprintln(w, "but protected processes may remain inaccessible. Unavailable sources are explicit.")
	fmt.Fprintln(w, "This is a snapshot of ports/descriptors, not message contents or an XPC service-name map.")
	fmt.Fprintln(w, "System V IPC and anonymous shared-memory mappings are outside this descriptor view.")
}

func renderIPCReport(w io.Writer, report ipc.Report, full bool) error {
	tw := output.NewTextWriter(w)
	if err := tw.Section("Process"); err != nil {
		return err
	}
	for _, kv := range [][2]string{
		{"PID", strconv.Itoa(report.Process.PID)},
		{"Name", report.Process.Name}, {"Path", report.Process.Path},
	} {
		if err := tw.KeyValue(kv[0], ipcText(kv[1], full)); err != nil {
			return err
		}
	}
	if err := renderIPCSource(tw, "Mach Ports", report.MachPorts.Source, report.MachPorts.Status, report.MachPorts.Warnings, full); err != nil {
		return err
	}
	if report.MachPorts.Status != "unavailable" {
		if err := tw.KeyValue("Ports Listed", strconv.Itoa(len(report.MachPorts.Ports))); err != nil {
			return err
		}
		table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "  Name\tRights\tIPC Object\tQueue\tQueued\tPeer / Object"); err != nil {
			return err
		}
		for _, port := range report.MachPorts.Ports {
			if err := renderMachPort(table, port, full, false); err != nil {
				return err
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
		if err := tw.Bullet("receive accepts messages; send/send_once sends; port_set groups receive rights; dead_name refers to a removed port"); err != nil {
			return err
		}
		if err := tw.Bullet("names are local to this task; Queue/Queued are queue limit/message count, '-' means unavailable; ports are not TCP/UDP numbers"); err != nil {
			return err
		}
	}
	if err := renderIPCSource(tw, "Local IPC", report.LocalIPC.Source, report.LocalIPC.Status, report.LocalIPC.Warnings, full); err != nil {
		return err
	}
	if report.LocalIPC.Status != "unavailable" {
		if len(report.LocalIPC.Files) == 0 {
			if err := tw.Bullet("no local IPC descriptors reported by lsof for this process"); err != nil {
				return err
			}
		} else {
			table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(table, "  FD\tKind\tAccess\tEndpoint"); err != nil {
				return err
			}
			for _, file := range report.LocalIPC.Files {
				if _, err := fmt.Fprintf(table, "  %s\t%s\t%s\t%s\n", ipcText(file.FD, full), file.Kind, fallback(file.Access, "-"), ipcText(file.Endpoint, full)); err != nil {
					return err
				}
			}
			if err := table.Flush(); err != nil {
				return err
			}
			if err := tw.Bullet("Access: r=read, w=write, u=read/write, '-'=unavailable; endpoints are native lsof names, not inferred peers"); err != nil {
				return err
			}
		}
	}
	if full && len(report.Raw) > 0 {
		if err := tw.Section("Raw Commands"); err != nil {
			return err
		}
		for _, result := range report.Raw {
			if err := tw.Bullet(ipcText(strings.Join(result.Command, " "), true)); err != nil {
				return err
			}
			for _, kv := range [][2]string{{"stdout", result.Stdout}, {"stderr", result.Stderr}} {
				if err := tw.Detail(kv[0] + ": " + ipcText(kv[1], true)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func renderIPCSource(tw *output.TextWriter, title, source, status string, warnings []string, full bool) error {
	if err := tw.Section(title); err != nil {
		return err
	}
	if err := tw.KeyValue("Source", ipcText(source, full)); err != nil {
		return err
	}
	if err := tw.KeyValue("Status", status); err != nil {
		return err
	}
	for _, warning := range warnings {
		if err := tw.Bullet(ipcText(warning, full)); err != nil {
			return err
		}
	}
	return nil
}

func renderMachPort(w io.Writer, port ipc.MachPort, full, member bool) error {
	name := port.Name
	if member {
		name = "member " + port.Identifier
	}
	peer := port.Type
	if port.TargetPID != nil {
		peer += fmt.Sprintf(" (%d) %s", *port.TargetPID, port.TargetProcess)
	}
	if port.Description != "" {
		peer += " " + port.Description
	}
	if _, err := fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\n", ipcText(name, full), ipcText(strings.Join(port.Rights, ","), full), ipcText(port.IPCObject, full), ipcCounter(port.QueueLimit), ipcCounter(port.MessageCount), ipcText(fallback(strings.TrimSpace(peer), "unknown"), full)); err != nil {
		return err
	}
	for _, child := range port.Members {
		if err := renderMachPort(w, child, full, true); err != nil {
			return err
		}
	}
	return nil
}

func ipcCounter(value *uint64) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatUint(*value, 10)
}

// Escape terminal controls even in --full output; JSON retains the raw values.
func ipcText(value string, full bool) string {
	if !full {
		value = diskHomePath.ReplaceAllString(value, "/Users/<redacted>")
	}
	var b strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			quoted := strconv.QuoteRune(r)
			b.WriteString(quoted[1 : len(quoted)-1])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
