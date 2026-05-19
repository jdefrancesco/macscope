package tcc

import (
	"strings"
)

type Report struct {
	Window   string    `json:"window,omitempty"`
	Events   []Event   `json:"events"`
	Findings []Finding `json:"findings,omitempty"`
	RawLines []string  `json:"raw_lines,omitempty"`
}

type Event struct {
	Timestamp string `json:"timestamp,omitempty"`
	Process   string `json:"process,omitempty"`
	Message   string `json:"message"`
	Service   string `json:"service,omitempty"`
	Decision  string `json:"decision,omitempty"`
}

type Finding struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
	Source     string   `json:"source"`
}

func Parse(window string, input string) Report {
	lines := nonEmptyLines(input)
	report := Report{
		Window:   window,
		RawLines: lines,
	}

	for _, line := range lines {
		event := parseLine(line)
		if !isTCCLine(event) {
			continue
		}
		report.Events = append(report.Events, event)
		if isDenial(event.Message) {
			report.Findings = append(report.Findings, Finding{
				Category:   "TCC_DENIAL",
				Severity:   "medium",
				Confidence: 0.82,
				Evidence:   []string{event.Message},
				Source:     "log show TCC predicate",
			})
		}
	}

	return report
}

func parseLine(line string) Event {
	fields := strings.Fields(line)
	event := Event{Message: strings.TrimSpace(line)}
	if len(fields) >= 4 {
		event.Timestamp = fields[0] + " " + fields[1]
		event.Process = strings.TrimSuffix(fields[3], ":")
	}
	if idx := strings.Index(line, "]:"); idx >= 0 {
		event.Message = strings.TrimSpace(line[idx+2:])
	}
	event.Service = detectService(line)
	event.Decision = detectDecision(line)
	return event
}

func isTCCLine(event Event) bool {
	lower := strings.ToLower(event.Message + " " + event.Process)
	return strings.Contains(lower, "tcc") ||
		strings.Contains(lower, "tccd") ||
		strings.Contains(lower, "ktcc") ||
		strings.Contains(lower, "privacy")
}

func detectDecision(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "deny") || strings.Contains(lower, "denied"):
		return "deny"
	case strings.Contains(lower, "allow") || strings.Contains(lower, "granted"):
		return "allow"
	default:
		return ""
	}
}

func detectService(line string) string {
	for _, marker := range []string{
		"kTCCServiceSystemPolicyAllFiles",
		"kTCCServiceDeveloperTool",
		"kTCCServiceCamera",
		"kTCCServiceMicrophone",
		"kTCCServiceScreenCapture",
		"kTCCServiceAccessibility",
		"kTCCServiceAppleEvents",
	} {
		if strings.Contains(line, marker) {
			return marker
		}
	}
	return ""
}

func isDenial(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "deny") || strings.Contains(lower, "denied")
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
