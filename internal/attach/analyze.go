package attach

import (
	"context"
	"os/user"
	"strings"
	"time"

	"github.com/jdefrancesco/macscope/internal/codesign"
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/logquery"
	"github.com/jdefrancesco/macscope/internal/process"
)

type Report struct {
	Process             process.Info        `json:"process"`
	CurrentUser         string              `json:"current_user,omitempty"`
	DeveloperGroup      GroupCheck          `json:"developer_group"`
	Signing             codesign.Inspection `json:"signing"`
	RecentLogLines      []string            `json:"recent_log_lines,omitempty"`
	Findings            []Finding           `json:"findings,omitempty"`
	InterpretationHints []string            `json:"interpretation_hints,omitempty"`
}

type GroupCheck struct {
	Group   string `json:"group"`
	Member  bool   `json:"member"`
	Message string `json:"message,omitempty"`
	Raw     string `json:"raw,omitempty"`
}

type Finding struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
	Source     string   `json:"source"`
}

func Analyze(ctx context.Context, pid int, last string, runner collect.Runner) (Report, error) {
	info, err := process.ForPID(ctx, pid, runner)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		Process: info,
		DeveloperGroup: GroupCheck{
			Group: "_developer",
		},
		InterpretationHints: []string{
			"taskgated or task_for_pid denial",
			"tccd denial involving Developer Tools, Debugging, Automation, or Full Disk Access",
			"amfid or syspolicyd signature or policy rejection",
			"target protected by platform policy, SIP, hardened runtime, or library validation",
		},
	}

	if current, err := user.Current(); err == nil {
		report.CurrentUser = current.Username
		report.DeveloperGroup = checkDeveloperGroup(ctx, current.Username, runner)
	}
	if info.Path != "" {
		report.Signing = codesign.InspectPath(ctx, info.Path, runner)
	}

	report.RecentLogLines = recentAttachLogs(ctx, last, runner)
	report.Findings = classify(report)

	return report, nil
}

func checkDeveloperGroup(ctx context.Context, username string, runner collect.Runner) GroupCheck {
	result, err := runner.Run(ctx, "dseditgroup", "-o", "checkmember", "-m", username, "_developer")
	raw := strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
	check := ParseGroupCheck("_developer", raw, result.ExitCode == 0)
	if err != nil && check.Message == "" {
		check.Message = err.Error()
	}
	return check
}

func ParseGroupCheck(group, raw string, exitOK bool) GroupCheck {
	lower := strings.ToLower(raw)
	member := exitOK && (strings.Contains(lower, "yes") || strings.Contains(lower, "is a member") || strings.Contains(lower, "true"))
	if strings.Contains(lower, "no") || strings.Contains(lower, "not a member") {
		member = false
	}
	return GroupCheck{
		Group:   group,
		Member:  member,
		Message: firstNonEmptyLine(raw),
		Raw:     raw,
	}
}

func recentAttachLogs(ctx context.Context, last string, runner collect.Runner) []string {
	if strings.TrimSpace(last) == "" {
		last = "30m"
	}
	result, err := runner.Run(ctx, "log", "show", "--last", last, "--style", "syslog", "--info", "--debug", "--predicate", logquery.AttachPredicate)
	if err != nil && result.Stdout == "" && result.Stderr == "" {
		return nil
	}
	return tailNonEmptyLinesFromSlice(filterAttachLogLines(result.Stdout+"\n"+result.Stderr), 25)
}

func classify(report Report) []Finding {
	var findings []Finding

	if !report.DeveloperGroup.Member && report.DeveloperGroup.Raw != "" {
		findings = append(findings, Finding{
			Category:   "DEBUGGER_GROUP_MISSING",
			Severity:   "medium",
			Confidence: 0.8,
			Evidence:   []string{"current user is not reported as a member of _developer"},
			Source:     "dseditgroup -o checkmember -m <user> _developer",
		})
	}

	if !report.Signing.Verification.Valid && report.Signing.Verification.Raw != "" {
		evidence := []string{"codesign verification returned a non-zero status"}
		if report.Signing.Verification.Message != "" {
			evidence = append(evidence, report.Signing.Verification.Message)
		}
		findings = append(findings, Finding{
			Category:   "INVALID_SIGNATURE",
			Severity:   "medium",
			Confidence: 0.82,
			Evidence:   evidence,
			Source:     "codesign --verify --deep --strict --verbose=4",
		})
	}

	if logEvidence := attachDenialEvidence(report.RecentLogLines); len(logEvidence) > 0 {
		findings = append(findings, Finding{
			Category:   "ATTACH_POLICY_DENIAL",
			Severity:   "medium",
			Confidence: 0.78,
			Evidence:   logEvidence,
			Source:     "log show attach predicate",
		})
	}

	return findings
}

func attachDenialEvidence(lines []string) []string {
	var evidence []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "log run noninteractively") {
			continue
		}
		hasDenial := strings.Contains(lower, "deny") ||
			strings.Contains(lower, "denied") ||
			strings.Contains(lower, "not permitted") ||
			strings.Contains(lower, "not allowed") ||
			strings.Contains(lower, "failed") ||
			strings.Contains(lower, "reject")
		hasAttachSignal := strings.Contains(lower, "task_for_pid") ||
			strings.Contains(lower, "taskgated") ||
			strings.Contains(lower, "tccd") ||
			strings.Contains(lower, "amfid") ||
			strings.Contains(lower, "syspolicyd") ||
			strings.Contains(lower, "debugserver") ||
			strings.Contains(lower, "developer tools")
		if hasDenial && hasAttachSignal {
			evidence = append(evidence, line)
		}
		if len(evidence) >= 5 {
			break
		}
	}
	return evidence
}

func filterAttachLogLines(input string) []string {
	var lines []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !isAttachRelevantLogLine(line) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func isAttachRelevantLogLine(line string) bool {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "log run noninteractively") || strings.HasPrefix(lower, "timestamp ") {
		return false
	}
	hasDecision := strings.Contains(lower, "deny") ||
		strings.Contains(lower, "denied") ||
		strings.Contains(lower, "not permitted") ||
		strings.Contains(lower, "not allowed") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "reject")
	hasActor := strings.Contains(lower, "taskgated") ||
		strings.Contains(lower, "amfid") ||
		strings.Contains(lower, "tccd") ||
		strings.Contains(lower, "syspolicyd")
	hasAttachSignal := strings.Contains(lower, "task_for_pid") ||
		strings.Contains(lower, "debugserver") ||
		strings.Contains(lower, "developer tools")
	if hasAttachSignal {
		return true
	}
	return hasActor && hasDecision
}

func tailNonEmptyLines(input string, limit int) []string {
	var lines []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) <= limit {
		return lines
	}
	return lines[len(lines)-limit:]
}

func tailNonEmptyLinesFromSlice(lines []string, limit int) []string {
	if len(lines) <= limit {
		return lines
	}
	return lines[len(lines)-limit:]
}

func firstNonEmptyLine(input string) string {
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func DefaultLogWindow() string {
	return (30 * time.Minute).String()
}
