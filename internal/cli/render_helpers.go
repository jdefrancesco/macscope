package cli

import (
	"fmt"
	"strings"

	"github.com/jdefrancesco/macscope/internal/output"
)

func renderFinding(tw *output.TextWriter, category, severity string, confidence float64, source string, details []string) error {
	normalizedSeverity := strings.ToUpper(strings.TrimSpace(severity))
	if normalizedSeverity == "" {
		normalizedSeverity = "UNKNOWN"
	}

	line := fmt.Sprintf("%s severity=%s confidence=%.2f", category, output.Level(normalizedSeverity), confidence)
	if source != "" {
		line += " source=" + source
	}
	if err := tw.Bullet(line); err != nil {
		return err
	}

	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail == "" {
			continue
		}
		if err := tw.Detail(detail); err != nil {
			return err
		}
	}

	return nil
}

func renderFindingWithEvidence(tw *output.TextWriter, category, severity string, confidence float64, source string, evidence []string) error {
	details := make([]string, 0, len(evidence))
	for _, item := range evidence {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		details = append(details, "evidence: "+item)
	}
	return renderFinding(tw, category, severity, confidence, source, details)
}
