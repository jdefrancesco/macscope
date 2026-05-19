package timeline

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jdefrancesco/macscope/internal/codesign"
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/logquery"
	"github.com/jdefrancesco/macscope/internal/process"
)

type Report struct {
	PID              int                 `json:"pid"`
	Window           string              `json:"window"`
	Process          process.Info        `json:"process"`
	Signing          codesign.Inspection `json:"signing"`
	Events           []Event             `json:"events"`
	Findings         []Finding           `json:"findings,omitempty"`
	CollectionErrors []string            `json:"collection_errors,omitempty"`
}

type Event struct {
	Time     string `json:"time,omitempty"`
	Source   string `json:"source"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Evidence string `json:"evidence,omitempty"`
}

type Finding struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
	Source     string   `json:"source"`
}

func Analyze(ctx context.Context, pid int, last string, runner collect.Runner) Report {
	if last == "" {
		last = "30m"
	}
	if runner.Timeout <= 0 {
		runner.Timeout = 45 * time.Second
	}

	report := Report{
		PID:    pid,
		Window: last,
	}

	info, err := process.ForPID(ctx, pid, runner)
	if err != nil {
		report.CollectionErrors = append(report.CollectionErrors, "process lookup: "+err.Error())
	} else {
		report.Process = info
		report.Events = append(report.Events, Event{
			Source:   "ps",
			Category: "PROCESS_OBSERVED",
			Severity: "info",
			Message:  fmt.Sprintf("pid=%d ppid=%d user=%s command=%s", info.PID, info.PPID, info.User, info.Command),
		})
		if info.Path != "" {
			report.Signing = codesign.InspectPath(ctx, info.Path, runner)
			report.Events = append(report.Events, signingEvents(report.Signing)...)
			report.Findings = append(report.Findings, signingFindings(report.Signing)...)
		}
	}

	logResult, err := logquery.Show(ctx, last, TimelinePredicate(pid), runner)
	if err != nil && logResult.Stdout == "" && logResult.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, "log show: "+err.Error())
	} else {
		logEvents := ParseLogEvents(logResult.Stdout + "\n" + logResult.Stderr)
		report.Events = append(report.Events, logEvents...)
		report.Findings = append(report.Findings, logFindings(logEvents)...)
	}

	sortEvents(report.Events)
	return report
}

func TimelinePredicate(pid int) string {
	pidText := strconv.Itoa(pid)
	return `(eventMessage CONTAINS[c] "pid ` + pidText + `" OR eventMessage CONTAINS[c] "[` + pidText + `]" OR eventMessage CONTAINS[c] "task_for_pid" OR eventMessage CONTAINS[c] "debug" OR process == "amfid" OR process == "tccd" OR process == "taskgated" OR process == "syspolicyd")`
}

func ParseLogEvents(input string) []Event {
	var events []Event
	for _, line := range nonEmptyLines(input) {
		if strings.HasPrefix(line, "Timestamp ") || strings.Contains(line, "log run noninteractively") {
			continue
		}
		if !isTimelineRelevant(line) {
			continue
		}
		event := Event{
			Time:     parseTimestamp(line),
			Source:   parseProcess(line),
			Category: classifyLogCategory(line),
			Severity: classifyLogSeverity(line),
			Message:  parseMessage(line),
			Evidence: line,
		}
		events = append(events, event)
	}
	return events
}

func signingEvents(signing codesign.Inspection) []Event {
	var events []Event
	if signing.Details.Raw != "" {
		message := "codesign details collected"
		if signing.Details.Identifier != "" {
			message += " identifier=" + signing.Details.Identifier
		}
		if signing.Details.TeamIdentifier != "" {
			message += " team_id=" + signing.Details.TeamIdentifier
		}
		events = append(events, Event{
			Source:   "codesign",
			Category: "SIGNING_DETAILS",
			Severity: "info",
			Message:  message,
		})
	}
	if signing.Verification.Raw != "" {
		severity := "info"
		category := "SIGNING_VALID"
		if !signing.Verification.Valid {
			severity = "medium"
			category = "SIGNING_INVALID"
		}
		events = append(events, Event{
			Source:   "codesign",
			Category: category,
			Severity: severity,
			Message:  signing.Verification.Message,
			Evidence: signing.Verification.Raw,
		})
	}
	return events
}

func signingFindings(signing codesign.Inspection) []Finding {
	if signing.Verification.Valid || signing.Verification.Raw == "" {
		return nil
	}
	evidence := []string{"codesign verification returned a non-zero status"}
	if signing.Verification.Message != "" {
		evidence = append(evidence, signing.Verification.Message)
	}
	return []Finding{
		{
			Category:   "INVALID_SIGNATURE",
			Severity:   "medium",
			Confidence: 0.82,
			Evidence:   evidence,
			Source:     "codesign --verify --deep --strict --verbose=4",
		},
	}
}

func logFindings(events []Event) []Finding {
	var findings []Finding
	for _, event := range events {
		if event.Severity != "medium" && event.Severity != "high" {
			continue
		}
		switch event.Category {
		case "ATTACH_POLICY_DENIAL", "TCC_DENIAL", "SIGNING_POLICY_EVENT":
			findings = append(findings, Finding{
				Category:   event.Category,
				Severity:   event.Severity,
				Confidence: 0.78,
				Evidence:   []string{event.Evidence},
				Source:     "log show timeline predicate",
			})
		}
	}
	return findings
}

func isTimelineRelevant(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "task_for_pid") ||
		strings.Contains(lower, "taskgated") ||
		strings.Contains(lower, "tccd") ||
		strings.Contains(lower, "amfid") ||
		strings.Contains(lower, "syspolicyd") ||
		strings.Contains(lower, "debugserver") ||
		strings.Contains(lower, "developer tools") ||
		strings.Contains(lower, "deny") ||
		strings.Contains(lower, "denied") ||
		strings.Contains(lower, "not permitted") ||
		strings.Contains(lower, "not allowed")
}

func classifyLogCategory(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "tccd") || strings.Contains(lower, "tcc") || strings.Contains(lower, "ktcc"):
		if hasDenial(lower) {
			return "TCC_DENIAL"
		}
		return "TCC_EVENT"
	case strings.Contains(lower, "task_for_pid") || strings.Contains(lower, "taskgated") || strings.Contains(lower, "debugserver"):
		if hasDenial(lower) {
			return "ATTACH_POLICY_DENIAL"
		}
		return "ATTACH_POLICY_EVENT"
	case strings.Contains(lower, "amfid") || strings.Contains(lower, "syspolicyd"):
		return "SIGNING_POLICY_EVENT"
	default:
		return "LOG_EVENT"
	}
}

func classifyLogSeverity(line string) string {
	lower := strings.ToLower(line)
	if hasDenial(lower) || strings.Contains(lower, "reject") || strings.Contains(lower, "failed") {
		return "medium"
	}
	return "info"
}

func hasDenial(lowerLine string) bool {
	return strings.Contains(lowerLine, "deny") ||
		strings.Contains(lowerLine, "denied") ||
		strings.Contains(lowerLine, "not permitted") ||
		strings.Contains(lowerLine, "not allowed")
}

func parseTimestamp(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return ""
	}
	if _, err := time.Parse("2006-01-02", fields[0]); err != nil {
		return ""
	}
	return fields[0] + " " + fields[1]
}

func parseProcess(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return "log"
	}
	return strings.TrimSuffix(fields[3], ":")
}

func parseMessage(line string) string {
	if idx := strings.Index(line, "]:"); idx >= 0 {
		return strings.TrimSpace(line[idx+2:])
	}
	return line
}

func sortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Time < events[j].Time
	})
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
