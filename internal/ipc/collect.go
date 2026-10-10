package ipc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/process"
)

type Options struct {
	Query  string
	Full   bool
	Runner process.CommandRunner
}

// Analyze takes a read-only snapshot. Source failures remain explicit in the
// report so an inaccessible Mach namespace does not hide available local IPC.
func Analyze(ctx context.Context, opts Options) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	if err := validateQuery(opts.Query); err != nil {
		return Report{}, err
	}
	runner := opts.Runner
	if runner == nil {
		native, err := collect.MacOSRunner()
		if err != nil {
			return Report{}, err
		}
		runner = native
	}
	info, err := process.Lookup(ctx, opts.Query, runner)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Report{}, ctxErr
	}
	if err != nil {
		return Report{}, fmt.Errorf("resolve IPC target: %w", err)
	}

	report := Report{Process: info}
	report.MachPorts, report.Raw = collectMach(ctx, info.PID, runner)
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	pid := strconv.Itoa(info.PID)
	result, runErr := runner.Run(ctx, "/usr/sbin/lsof", "-nP", "-a", "-p", pid, "-F0pftan")
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	report.Raw = append(report.Raw, result)
	report.LocalIPC = LocalReport{
		Source: "/usr/sbin/lsof -nP -a -p " + pid + " -F0pftan",
		Status: "unavailable", Files: []LocalFile{},
	}
	files, parseErr := ParseLocalIPC(result.Stdout, info.PID)
	if parseErr == nil {
		report.LocalIPC.Files = files
		report.LocalIPC.Status = "ok"
	}
	if runErr != nil {
		report.LocalIPC.Warnings = append(report.LocalIPC.Warnings, "local IPC collection: "+commandFailure(result, runErr)+"; the process may have exited or access may be restricted")
	} else if parseErr != nil {
		report.LocalIPC.Warnings = append(report.LocalIPC.Warnings, parseErr.Error())
	}
	if detail := strings.TrimSpace(result.Stderr); runErr == nil && detail != "" {
		report.LocalIPC.Warnings = append(report.LocalIPC.Warnings, detail)
	}
	if report.LocalIPC.Status == "ok" && len(report.LocalIPC.Warnings) > 0 {
		report.LocalIPC.Status = "partial"
	}
	if !opts.Full {
		report.Raw = nil
	}
	return report, nil
}

func collectMach(ctx context.Context, pid int, runner process.CommandRunner) (MachReport, []collect.Result) {
	pidText := strconv.Itoa(pid)
	report := MachReport{
		Source: "/usr/bin/lsmp -p " + pidText + " -j <temporary-file>",
		Status: "unavailable", Ports: []MachPort{},
	}
	file, err := os.CreateTemp("", "macscope-lsmp-*.json")
	if err != nil {
		report.Warnings = append(report.Warnings, "prepare lsmp output: "+err.Error())
		return report, nil
	}
	path := file.Name()
	defer os.Remove(path) // Best-effort removal of our private, temporary evidence file.
	if err := file.Close(); err != nil {
		report.Warnings = append(report.Warnings, "close lsmp output file: "+err.Error())
		return report, nil
	}
	result, runErr := runner.Run(ctx, "/usr/bin/lsmp", "-p", pidText, "-j", path)
	raw := []collect.Result{result}
	if runErr != nil {
		report.Warnings = append(report.Warnings, "Mach port collection: "+commandFailure(result, runErr))
		report.Warnings = append(report.Warnings, "lsmp requires task-read access; root can improve visibility, but protected processes may remain inaccessible")
		return report, raw
	}
	data, err := os.ReadFile(path)
	if err == nil {
		var parsed MachReport
		parsed, err = ParseMachPorts(data, pid)
		if err == nil {
			parsed.Source = report.Source
			report = parsed
		}
	}
	if err != nil {
		report.Warnings = append(report.Warnings, "Mach port collection: "+err.Error())
	}
	if detail := strings.TrimSpace(result.Stderr); detail != "" {
		report.Warnings = append(report.Warnings, detail)
	}
	if report.Status == "ok" && len(report.Warnings) > 0 {
		report.Status = "partial"
	}
	return report, raw
}

func commandFailure(result collect.Result, err error) string {
	message := err.Error()
	if detail := strings.TrimSpace(result.Stderr); detail != "" {
		message += ": " + detail
	}
	return message
}

func validateQuery(query string) error {
	if strings.TrimSpace(query) == "" || strings.HasPrefix(query, "-") {
		return fmt.Errorf("supply a positive PID or a process name that does not start with '-'")
	}
	if strings.Trim(query, "0123456789") == "" {
		if _, ok := process.ParsePID(query); !ok {
			return fmt.Errorf("invalid PID: %s; supply a positive PID", query)
		}
	}
	return nil
}
