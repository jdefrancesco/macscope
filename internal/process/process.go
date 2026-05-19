package process

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) (collect.Result, error)
}

type Info struct {
	PID     int    `json:"pid"`
	PPID    int    `json:"ppid"`
	User    string `json:"user,omitempty"`
	Group   string `json:"group,omitempty"`
	Stat    string `json:"stat,omitempty"`
	Comm    string `json:"comm,omitempty"`
	Command string `json:"command,omitempty"`
	Name    string `json:"name,omitempty"`
	Path    string `json:"path,omitempty"`
}

func Lookup(ctx context.Context, query string, runner CommandRunner) (Info, error) {
	if query == "" {
		return Info{}, errors.New("empty process query")
	}
	if runner == nil {
		runner = collect.Runner{}
	}

	if pid, ok := ParsePID(query); ok {
		return ForPID(ctx, pid, runner)
	}

	pid, err := newestPID(ctx, runner, "-x", query)
	if err != nil {
		pid, err = newestPID(ctx, runner, "-f", query)
	}
	if err != nil {
		return Info{}, fmt.Errorf("no matching process: %s", query)
	}

	return ForPID(ctx, pid, runner)
}

func ForPID(ctx context.Context, pid int, runner CommandRunner) (Info, error) {
	if runner == nil {
		runner = collect.Runner{}
	}
	if pid <= 0 {
		return Info{}, fmt.Errorf("invalid pid: %d", pid)
	}

	result, err := runner.Run(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "pid=", "-o", "ppid=", "-o", "user=", "-o", "group=", "-o", "stat=", "-o", "comm=", "-o", "command=")
	if err != nil {
		return Info{}, err
	}

	info, err := ParsePSLine(result.Stdout)
	if err != nil {
		return Info{}, err
	}
	return info, nil
}

func ParsePID(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	pid, err := strconv.Atoi(value)
	return pid, err == nil && pid > 0
}

func ParsePSLine(output string) (Info, error) {
	line := firstNonEmptyLine(output)
	if line == "" {
		return Info{}, errors.New("ps returned no process rows")
	}

	fields := strings.Fields(line)
	if len(fields) < 7 {
		return Info{}, fmt.Errorf("unexpected ps output: %q", line)
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return Info{}, fmt.Errorf("parse pid: %w", err)
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return Info{}, fmt.Errorf("parse ppid: %w", err)
	}

	comm := fields[5]
	command := strings.Join(fields[6:], " ")
	info := Info{
		PID:     pid,
		PPID:    ppid,
		User:    fields[2],
		Group:   fields[3],
		Stat:    fields[4],
		Comm:    comm,
		Command: command,
		Name:    processName(comm, command),
		Path:    processPath(comm, command),
	}

	return info, nil
}

func newestPID(ctx context.Context, runner CommandRunner, mode string, query string) (int, error) {
	result, err := runner.Run(ctx, "pgrep", "-n", mode, query)
	if err != nil {
		return 0, err
	}
	pidText := firstNonEmptyLine(result.Stdout)
	pid, ok := ParsePID(pidText)
	if !ok {
		return 0, fmt.Errorf("pgrep returned invalid pid: %q", pidText)
	}
	return pid, nil
}

func firstNonEmptyLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func processName(comm, command string) string {
	if comm != "" {
		return filepath.Base(comm)
	}
	if first := firstCommandToken(command); first != "" {
		return filepath.Base(first)
	}
	return ""
}

func processPath(comm, command string) string {
	if first := firstCommandToken(command); strings.HasPrefix(first, "/") {
		return first
	}
	if strings.HasPrefix(comm, "/") {
		return comm
	}
	return ""
}

func firstCommandToken(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], `"'`)
}
