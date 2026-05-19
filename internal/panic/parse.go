package paniclog

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	TypeUnknown         = "UNKNOWN"
	TypeKernelPanic     = "KERNEL_PANIC"
	TypeWatchdogTimeout = "WATCHDOG_TIMEOUT"
)

type Report struct {
	SourcePath         string   `json:"source_path,omitempty"`
	PanicType          string   `json:"panic_type"`
	Summary            string   `json:"summary,omitempty"`
	PanicString        string   `json:"panic_string,omitempty"`
	WatchdogTimeoutSec *int     `json:"watchdog_timeout_sec,omitempty"`
	CPU                *int     `json:"cpu,omitempty"`
	CallerAddress      string   `json:"caller_address,omitempty"`
	OSVersion          string   `json:"os_version,omitempty"`
	BootSessionUUID    string   `json:"boot_session_uuid,omitempty"`
	CurrentPhase       string   `json:"current_phase,omitempty"`
	SavedReportPath    string   `json:"saved_report_path,omitempty"`
	SOCDPresent        *bool    `json:"socd_present,omitempty"`
	PreOSMarkers       []string `json:"pre_os_markers,omitempty"`
	Indicators         []string `json:"indicators,omitempty"`
	SuspectedCauses    []Cause  `json:"suspected_causes,omitempty"`
	Evidence           []string `json:"evidence,omitempty"`
}

type Cause struct {
	Category   string   `json:"category"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
}

var (
	panicLineRe = regexp.MustCompile(`(?im)panic\(cpu ([0-9]+).*?\): (.*)`)
	watchdogRe  = regexp.MustCompile(`(?im)watchdog timeout: no checkins from watchdogd in ([0-9]+) seconds`)
	bootUUIDRe  = regexp.MustCompile(`(?im)bootsessionuuid: ([A-Fa-f0-9\-]+)`)
	osVersionRe = regexp.MustCompile(`(?im)osversion: ([^,\n]+)`)
	phaseRe     = regexp.MustCompile(`(?m)Current Phase = "([^"]+)"`)
	savedRe     = regexp.MustCompile(`(?m)Saved type '210.*' report .* at (/Library/Logs/DiagnosticReports/[^ ]+\.panic)`)
	callerRe    = regexp.MustCompile(`(?i)\bcaller\s+(0x[0-9a-f]+)`)
)

var displayDockIndicators = []string{
	"AppleDisplayCrossbar",
	"AppleATCDPINAdapterPort",
	"DisplayPort",
	"Thunderbolt",
	"IOGPU",
	"AGX",
	"USB-C",
}

var softwareUpdateIndicators = []string{
	"Installer Progress",
	"softwareupdated",
	"IOKit Boot",
	"PanicMedic",
}

var preOSMarkers = []string{
	"iBoot",
	"pre-OS",
	"PanicMedic",
}

func Parse(sourcePath string, input string) Report {
	report := Report{
		SourcePath:  sourcePath,
		PanicType:   TypeUnknown,
		SOCDPresent: boolPtr(strings.Contains(strings.ToLower(input), "socd")),
	}

	if match := panicLineRe.FindStringSubmatch(input); len(match) == 3 {
		if cpu, err := strconv.Atoi(match[1]); err == nil {
			report.CPU = &cpu
		}
		report.PanicString = strings.TrimSpace(match[2])
		report.PanicType = TypeKernelPanic
		report.Evidence = append(report.Evidence, "panic string found")
	}

	if match := watchdogRe.FindStringSubmatch(input); len(match) == 2 {
		timeout, err := strconv.Atoi(match[1])
		if err == nil {
			report.WatchdogTimeoutSec = &timeout
			report.PanicType = TypeWatchdogTimeout
			report.Evidence = append(report.Evidence, "watchdogd missed checkins for "+match[1]+" seconds")
		}
	}

	if report.PanicType != TypeWatchdogTimeout && strings.Contains(strings.ToLower(report.PanicString), "watchdog timeout") {
		report.PanicType = TypeWatchdogTimeout
		report.Evidence = append(report.Evidence, "panic string contains watchdog timeout")
	}

	if match := bootUUIDRe.FindStringSubmatch(input); len(match) == 2 {
		report.BootSessionUUID = strings.TrimSpace(match[1])
	}
	if match := osVersionRe.FindStringSubmatch(input); len(match) == 2 {
		report.OSVersion = strings.TrimSpace(match[1])
	}
	if match := phaseRe.FindStringSubmatch(input); len(match) == 2 {
		report.CurrentPhase = strings.TrimSpace(match[1])
	}
	if match := savedRe.FindStringSubmatch(input); len(match) == 2 {
		report.SavedReportPath = strings.TrimSpace(match[1])
	}
	if match := callerRe.FindStringSubmatch(input); len(match) == 2 {
		report.CallerAddress = strings.TrimSpace(match[1])
	}

	report.Indicators = detectIndicators(input)
	report.PreOSMarkers = detectMarkers(input, preOSMarkers)
	report.SuspectedCauses = classify(report, input)
	report.Summary = summarize(report)

	return report
}

func classify(report Report, input string) []Cause {
	if report.PanicType != TypeWatchdogTimeout {
		return nil
	}

	var causes []Cause
	if containsAny(input, []string{"IOKit Boot", "Current Phase"}) {
		evidence := []string{"IOKit Boot phase detected near panic evidence"}
		if report.CurrentPhase != "" {
			evidence = append(evidence, "Current Phase = "+report.CurrentPhase)
		}
		causes = append(causes, Cause{
			Category:   "BOOT_IOKIT_STALL",
			Confidence: 0.81,
			Evidence:   evidence,
		})
	}

	displayIndicators := matchingIndicators(input, displayDockIndicators)
	if len(displayIndicators) > 0 {
		causes = append(causes, Cause{
			Category:   "EXTERNAL_DISPLAY_OR_DOCK",
			Confidence: 0.72,
			Evidence:   prefixEvidence("display/dock indicator: ", displayIndicators),
		})
	}

	return causes
}

func summarize(report Report) string {
	switch report.PanicType {
	case TypeWatchdogTimeout:
		if report.WatchdogTimeoutSec != nil {
			return "watchdogd failed to check in for " + strconv.Itoa(*report.WatchdogTimeoutSec) + " seconds, indicating a system-wide stall."
		}
		return "panic string indicates a watchdog timeout, suggesting a system-wide stall."
	case TypeKernelPanic:
		return "kernel panic string detected."
	default:
		return "no kernel panic string detected."
	}
}

func detectIndicators(input string) []string {
	indicators := matchingIndicators(input, append(displayDockIndicators, softwareUpdateIndicators...))
	return uniqueStrings(indicators)
}

func detectMarkers(input string, markers []string) []string {
	return uniqueStrings(matchingIndicators(input, markers))
}

func matchingIndicators(input string, indicators []string) []string {
	lowerInput := strings.ToLower(input)
	var out []string
	for _, indicator := range indicators {
		if strings.Contains(lowerInput, strings.ToLower(indicator)) {
			out = append(out, indicator)
		}
	}
	return out
}

func containsAny(input string, needles []string) bool {
	return len(matchingIndicators(input, needles)) > 0
}

func prefixEvidence(prefix string, values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, prefix+value)
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func boolPtr(value bool) *bool {
	return &value
}
