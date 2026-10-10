// Package ipc collects a process's Mach port namespace and local IPC descriptors.
package ipc

import (
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/process"
)

type Report struct {
	Process   process.Info     `json:"process"`
	MachPorts MachReport       `json:"mach_ports"`
	LocalIPC  LocalReport      `json:"local_ipc"`
	Raw       []collect.Result `json:"raw,omitempty"`
}

// Status is ok, partial, or unavailable. An unavailable source is never an
// assertion that the process has no IPC endpoints.
type MachReport struct {
	Source        string     `json:"source"`
	Status        string     `json:"status"`
	ReportedTotal *int       `json:"reported_total,omitempty"`
	Ports         []MachPort `json:"ports"`
	Warnings      []string   `json:"warnings,omitempty"`
}

type MachPort struct {
	Name           string     `json:"name"`
	IPCObject      string     `json:"ipc_object"`
	Type           string     `json:"type,omitempty"`
	Rights         []string   `json:"rights"`
	Flags          []string   `json:"flags,omitempty"`
	ReceiveRights  *uint64    `json:"receive_rights,omitempty"`
	SendRights     *uint64    `json:"send_rights,omitempty"`
	SendOnceRights *uint64    `json:"send_once_rights,omitempty"`
	QueueLimit     *uint64    `json:"queue_limit,omitempty"`
	MessageCount   *uint64    `json:"message_count,omitempty"`
	Identifier     string     `json:"identifier,omitempty"`
	TargetPID      *int       `json:"target_pid,omitempty"`
	TargetProcess  string     `json:"target_process,omitempty"`
	Description    string     `json:"description,omitempty"`
	Members        []MachPort `json:"members,omitempty"`
}

type LocalReport struct {
	Source   string      `json:"source"`
	Status   string      `json:"status"`
	Files    []LocalFile `json:"files"`
	Warnings []string    `json:"warnings,omitempty"`
}

type LocalFile struct {
	FD       string `json:"fd"`
	Kind     string `json:"kind"`
	Type     string `json:"type"`
	Access   string `json:"access,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}
