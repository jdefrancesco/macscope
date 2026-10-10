package ipc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// lsmpPort describes Apple's lsmp -j schema, rather than the CLI's JSON schema.
type lsmpPort struct {
	Name           string     `json:"name"`
	IPCObject      string     `json:"ipc-object"`
	Type           string     `json:"type"`
	Rights         []string   `json:"rights"`
	Flags          []string   `json:"port_status_flags"`
	ReceiveRights  *uint64    `json:"recv_rights"`
	SendRights     *uint64    `json:"send_rights"`
	SendOnceRights *uint64    `json:"send_once_rights"`
	QueueLimit     *uint64    `json:"queue_limit"`
	MessageCount   *uint64    `json:"msg_count"`
	Identifier     string     `json:"identifier"`
	PID            *int       `json:"pid"`
	Process        string     `json:"process"`
	Description    string     `json:"description"`
	Members        []lsmpPort `json:"members"`
}

// ParseMachPorts selects only the requested task and retains unknown counters
// as absent fields. Port-set members are nested and do not inflate row counts.
func ParseMachPorts(data []byte, pid int) (MachReport, error) {
	var native struct {
		Processes []struct {
			PID   int        `json:"pid"`
			Total *int       `json:"total"`
			Ports []lsmpPort `json:"ports"`
		} `json:"processes"`
	}
	if err := json.Unmarshal(data, &native); err != nil {
		return MachReport{}, fmt.Errorf("parse lsmp JSON: %w", err)
	}
	for _, task := range native.Processes {
		if task.PID != pid {
			continue
		}
		if task.Ports == nil {
			return MachReport{}, fmt.Errorf("lsmp returned no port array for PID %d", pid)
		}
		if task.Total != nil && *task.Total < 0 {
			return MachReport{}, fmt.Errorf("lsmp returned a negative entry count for PID %d", pid)
		}
		ports := make([]MachPort, 0, len(task.Ports))
		for _, port := range task.Ports {
			if port.Name == "" || port.IPCObject == "" {
				return MachReport{}, fmt.Errorf("lsmp returned a port without a name or IPC object for PID %d", pid)
			}
			ports = append(ports, convertMachPort(port))
		}
		report := MachReport{Status: "ok", ReportedTotal: task.Total, Ports: ports}
		if task.Total != nil && *task.Total != len(ports) {
			report.Status = "partial"
			report.Warnings = append(report.Warnings, fmt.Sprintf("lsmp reported %d entries but listed %d ports; some port details were unavailable", *task.Total, len(ports)))
		}
		return report, nil
	}
	return MachReport{}, fmt.Errorf("lsmp returned no task for PID %d", pid)
}

func convertMachPort(port lsmpPort) MachPort {
	rights := make([]string, 0, len(port.Rights))
	for _, right := range port.Rights {
		switch right {
		case "recv":
			right = "receive"
		case "send once":
			right = "send_once"
		}
		rights = append(rights, right)
	}
	switch port.Type {
	case "port set":
		rights = append(rights, "port_set")
	case "dead name":
		rights = append(rights, "dead_name")
	}
	members := make([]MachPort, 0, len(port.Members))
	for _, member := range port.Members {
		members = append(members, convertMachPort(member))
	}
	pid := port.PID
	peer := port.Process
	if pid == nil || *pid <= 0 {
		pid = nil
		peer = ""
	}
	return MachPort{
		Name: port.Name, IPCObject: port.IPCObject, Type: port.Type, Rights: rights,
		Flags: port.Flags, ReceiveRights: port.ReceiveRights, SendRights: port.SendRights,
		SendOnceRights: port.SendOnceRights, QueueLimit: port.QueueLimit, MessageCount: port.MessageCount,
		Identifier: port.Identifier, TargetPID: pid, TargetProcess: peer, Description: port.Description, Members: members,
	}
}

// ParseLocalIPC reads lsof -F0pftan output. NUL-delimited fields keep paths with
// spaces or newlines intact; regular files and Internet sockets are excluded.
func ParseLocalIPC(data string, pid int) ([]LocalFile, error) {
	files := make([]LocalFile, 0)
	currentPID := 0
	sawPID := false
	var current LocalFile
	flush := func() error {
		if current.FD != "" && current.Type == "" {
			return fmt.Errorf("lsof returned descriptor %q without a file type", current.FD)
		}
		if currentPID == pid && current.FD != "" {
			if kind := ipcKind(current.Type); kind != "" {
				current.Kind = kind
				files = append(files, current)
			}
		}
		current = LocalFile{}
		return nil
	}
	if data == "" {
		return nil, fmt.Errorf("lsof returned no process record for PID %d", pid)
	}
	if !strings.ContainsRune(data, '\x00') || !strings.HasSuffix(data, "\x00\n") && !strings.HasSuffix(data, "\x00") {
		return nil, fmt.Errorf("expected complete NUL-delimited lsof field output")
	}
	for _, field := range strings.Split(data, "\x00") {
		// lsof emits a newline after each process or file set, outside fields.
		field = strings.TrimPrefix(field, "\n")
		if field == "" {
			continue
		}
		value := field[1:]
		switch field[0] {
		case 'p':
			if err := flush(); err != nil {
				return nil, err
			}
			var err error
			currentPID, err = strconv.Atoi(value)
			if err != nil || currentPID <= 0 {
				return nil, fmt.Errorf("lsof returned invalid PID %q", value)
			}
			sawPID = sawPID || currentPID == pid
		case 'f':
			if currentPID == 0 || value == "" {
				return nil, fmt.Errorf("lsof returned a descriptor without a process record or FD")
			}
			if err := flush(); err != nil {
				return nil, err
			}
			current.FD = value
		case 't':
			if current.FD == "" {
				return nil, fmt.Errorf("lsof returned a file type without a descriptor")
			}
			current.Type = value
		case 'a':
			current.Access = strings.TrimSpace(value)
		case 'n':
			current.Endpoint = value
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if !sawPID {
		return nil, fmt.Errorf("lsof returned no process record for PID %d", pid)
	}
	return files, nil
}

func ipcKind(fileType string) string {
	switch fileType {
	case "unix":
		return "unix_socket"
	case "PIPE":
		return "pipe"
	case "FIFO":
		return "fifo"
	case "PSXSHM":
		return "shared_memory"
	case "PSXSEM":
		return "semaphore"
	default:
		return ""
	}
}
