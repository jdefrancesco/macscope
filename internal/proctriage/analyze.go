package proctriage

import (
	"context"

	"github.com/jdefrancesco/macscope/internal/codesign"
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/process"
)

type Report struct {
	Process   process.Info        `json:"process"`
	Signing   codesign.Inspection `json:"signing"`
	Findings  []Finding           `json:"findings,omitempty"`
	NextSteps []string            `json:"next_steps,omitempty"`
}

type Finding struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
	Source     string   `json:"source"`
}

func Analyze(ctx context.Context, query string, runner collect.Runner) (Report, error) {
	info, err := process.Lookup(ctx, query, runner)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		Process: info,
		NextSteps: []string{
			"use macscope attach <pid> for debugger attach checks",
			"use sample or vmmap manually when deeper runtime state is needed",
		},
	}
	if info.Path != "" {
		report.Signing = codesign.InspectPath(ctx, info.Path, runner)
		report.Findings = signingFindings(report.Signing)
	}

	return report, nil
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
