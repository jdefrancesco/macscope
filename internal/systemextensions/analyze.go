package systemextensions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jdefrancesco/macscope/internal/collect"
)

// Report is the result of a system extension inventory.
type Report struct {
	Extensions       []Extension `json:"extensions"`
	Findings         []Finding   `json:"findings,omitempty"`
	CollectionErrors []string    `json:"collection_errors,omitempty"`
	Raw              string      `json:"raw,omitempty"`
}

// Finding is an evidence-backed classification.
type Finding struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
	Source     string   `json:"source"`
}

// Analyze runs systemextensionsctl list and classifies the results.
func Analyze(ctx context.Context, runner collect.Runner) Report {
	if runner.Timeout <= 0 {
		runner.Timeout = 20 * time.Second
	}

	var report Report

	result, err := runner.Run(ctx, "systemextensionsctl", "list")
	if err != nil && result.Stdout == "" && result.Stderr == "" {
		report.CollectionErrors = append(report.CollectionErrors, err.Error())
		return report
	}
	raw := strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
	report.Raw = raw
	report.Extensions = ParseList(raw)
	report.Findings = Classify(report.Extensions)
	return report
}

// Classify produces findings from a slice of extensions.
func Classify(extensions []Extension) []Finding {
	var findings []Finding

	var networkActive []Extension
	for _, ext := range extensions {
		if ext.Type == TypeNetwork && ext.Enabled {
			networkActive = append(networkActive, ext)
		}
	}
	if len(networkActive) > 1 {
		var evidence []string
		for _, ext := range networkActive {
			evidence = append(evidence, fmt.Sprintf("%s (%s)", ext.BundleID, ext.Name))
		}
		findings = append(findings, Finding{
			Category:   "MULTIPLE_NETWORK_EXTENSIONS",
			Severity:   "medium",
			Confidence: 0.80,
			Evidence:   evidence,
			Source:     "systemextensionsctl list",
		})
	}

	for _, ext := range extensions {
		state := strings.ToLower(ext.State)
		if ext.Enabled && !strings.Contains(state, "activated enabled") {
			cat := "EXTENSION_NOT_ACTIVATED"
			sev := "medium"
			conf := 0.85
			if strings.Contains(state, "waiting for user") {
				cat = "EXTENSION_AWAITING_APPROVAL"
				sev = "low"
				conf = 0.90
			} else if strings.Contains(state, "terminated") {
				cat = "EXTENSION_TERMINATED"
				sev = "high"
				conf = 0.88
			}
			findings = append(findings, Finding{
				Category:   cat,
				Severity:   sev,
				Confidence: conf,
				Evidence: []string{
					fmt.Sprintf("%s name=%q state=%q", ext.BundleID, ext.Name, ext.State),
				},
				Source: "systemextensionsctl list",
			})
		}
	}

	return findings
}
