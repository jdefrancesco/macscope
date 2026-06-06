package vpn

import (
	"context"
	"strings"
	"time"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/logquery"
	"github.com/jdefrancesco/macscope/internal/pmset"
)

type Report struct {
	RequestedName     string      `json:"requested_name,omitempty"`
	LogWindow         string      `json:"log_window"`
	Services          []Service   `json:"services,omitempty"`
	SelectedStatus    string      `json:"selected_status,omitempty"`
	Interfaces        []Interface `json:"interfaces,omitempty"`
	DNS               string      `json:"dns,omitempty"`
	Proxy             string      `json:"proxy,omitempty"`
	DefaultRoute      string      `json:"default_route,omitempty"`
	RouteTable        string      `json:"route_table,omitempty"`
	InterfaceCounters string      `json:"interface_counters,omitempty"`
	RecentLogLines    []string    `json:"recent_log_lines,omitempty"`
	SleepWakeLines    []string    `json:"sleep_wake_lines,omitempty"`
	Findings          []Finding   `json:"findings,omitempty"`
	CollectionErrors  []string    `json:"collection_errors,omitempty"`
}

type Service struct {
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	Raw       string `json:"raw"`
	Connected bool   `json:"connected"`
}

type Interface struct {
	Name      string   `json:"name"`
	Status    string   `json:"status,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	Raw       []string `json:"raw,omitempty"`
}

type Finding struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
	Source     string   `json:"source"`
}

func Analyze(ctx context.Context, vpnName string, last string, runner collect.Runner) Report {
	if last == "" {
		last = "60m"
	}
	if runner.Timeout <= 0 {
		runner.Timeout = 30 * time.Second
	}

	report := Report{
		RequestedName: vpnName,
		LogWindow:     last,
	}

	if result, err := runner.Run(ctx, "scutil", "--nc", "list"); err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
	} else {
		report.Services = ParseServices(result.Stdout + "\n" + result.Stderr)
	}

	if vpnName != "" {
		if result, err := runner.Run(ctx, "scutil", "--nc", "status", vpnName); err != nil && result.Stdout == "" && result.Stderr == "" {
			report.CollectionErrors = append(report.CollectionErrors, err.Error())
		} else {
			report.SelectedStatus = strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
		}
	}

	if result, err := runner.Run(ctx, "scutil", "--dns"); err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
	} else {
		report.DNS = strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
	}

	if result, err := runner.Run(ctx, "scutil", "--proxy"); err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
	} else {
		report.Proxy = strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
	}

	if result, err := runner.Run(ctx, "ifconfig"); err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
	} else {
		report.Interfaces = ParseInterfaces(result.Stdout + "\n" + result.Stderr)
	}

	if result, err := runner.Run(ctx, "route", "-n", "get", "default"); err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
	} else {
		report.DefaultRoute = strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
	}

	if result, err := runner.Run(ctx, "netstat", "-rn"); err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
	} else {
		report.RouteTable = strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
	}

	if result, err := runner.Run(ctx, "netstat", "-i"); err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
	} else {
		report.InterfaceCounters = strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
	}

	if result, err := logquery.Show(ctx, last, logquery.VPNPredicate, runner); err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
	} else {
		report.RecentLogLines = Tail(FilterVPNLogLines(result.Stdout+"\n"+result.Stderr), 80)
	}

	if result, err := runner.Run(ctx, "pmset", "-g", "log"); err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
	} else {
		events := pmset.Tail(pmset.SleepWakeEvents(pmset.ParseLog(result.Stdout+"\n"+result.Stderr)), 60)
		report.SleepWakeLines = make([]string, 0, len(events))
		for _, event := range events {
			report.SleepWakeLines = append(report.SleepWakeLines, event.String())
		}
	}

	report.Findings = Classify(report)
	return report
}

func ParseServices(input string) []Service {
	var services []Service
	for _, line := range nonEmptyLines(input) {
		status := ""
		name := ""
		if start := strings.Index(line, "("); start >= 0 {
			if end := strings.Index(line[start:], ")"); end >= 0 {
				status = strings.TrimSpace(line[start+1 : start+end])
				name = strings.TrimSpace(line[start+end+1:])
			}
		}
		if name == "" {
			name = strings.Trim(strings.TrimSpace(line), `"`)
		}
		services = append(services, Service{
			Name:      strings.Trim(name, `"`),
			Status:    status,
			Raw:       line,
			Connected: strings.EqualFold(status, "connected"),
		})
	}
	return services
}

func ParseInterfaces(input string) []Interface {
	var interfaces []Interface
	var current *Interface
	for _, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") && strings.Contains(line, ": flags=") {
			name := strings.TrimSpace(strings.SplitN(line, ":", 2)[0])
			if strings.HasPrefix(name, "utun") {
				interfaces = append(interfaces, Interface{Name: name, Raw: []string{strings.TrimSpace(line)}})
				current = &interfaces[len(interfaces)-1]
			} else {
				current = nil
			}
			continue
		}
		if current == nil {
			continue
		}
		trimmed := strings.TrimSpace(line)
		current.Raw = append(current.Raw, trimmed)
		if strings.HasPrefix(trimmed, "status:") {
			current.Status = strings.TrimSpace(strings.TrimPrefix(trimmed, "status:"))
		}
		if strings.HasPrefix(trimmed, "inet ") || strings.HasPrefix(trimmed, "inet6 ") {
			current.Addresses = append(current.Addresses, trimmed)
		}
	}
	return interfaces
}

func FilterVPNLogLines(input string) []string {
	var lines []string
	for _, line := range nonEmptyLines(input) {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "vpn") ||
			strings.Contains(lower, "utun") ||
			strings.Contains(lower, "ipsec") ||
			strings.Contains(lower, "ike") ||
			strings.Contains(lower, "disconnect") {
			lines = append(lines, line)
		}
	}
	return lines
}

func Tail(lines []string, limit int) []string {
	if len(lines) <= limit {
		return lines
	}
	return lines[len(lines)-limit:]
}

func Classify(report Report) []Finding {
	var findings []Finding

	if report.RequestedName != "" && report.SelectedStatus != "" {
		lower := strings.ToLower(report.SelectedStatus)
		if strings.Contains(lower, "disconnected") || strings.Contains(lower, "not connected") || strings.Contains(lower, "invalid") {
			findings = append(findings, Finding{
				Category:   "VPN_SERVICE_DISCONNECTED",
				Severity:   "medium",
				Confidence: 0.78,
				Evidence:   []string{firstLine(report.SelectedStatus)},
				Source:     "scutil --nc status",
			})
		}
	}

	if report.RequestedName != "" && len(report.Interfaces) == 0 {
		findings = append(findings, Finding{
			Category:   "UTUN_INTERFACE_ABSENT",
			Severity:   "low",
			Confidence: 0.62,
			Evidence:   []string{"no utun interfaces were parsed from ifconfig output"},
			Source:     "ifconfig",
		})
	}

	if evidence := vpnErrorEvidence(report.RecentLogLines); len(evidence) > 0 {
		findings = append(findings, Finding{
			Category:   "VPN_LOG_ERROR_OR_DISCONNECT",
			Severity:   "medium",
			Confidence: 0.76,
			Evidence:   evidence,
			Source:     "log show VPN predicate",
		})
	}

	return findings
}

func vpnErrorEvidence(lines []string) []string {
	var evidence []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "disconnect") ||
			strings.Contains(lower, "failed") ||
			strings.Contains(lower, "error") ||
			strings.Contains(lower, "timeout") {
			evidence = append(evidence, line)
		}
		if len(evidence) >= 5 {
			break
		}
	}
	return evidence
}

func firstLine(input string) string {
	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func nonEmptyLines(input string) []string {
	var lines []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
