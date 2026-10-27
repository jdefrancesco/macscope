package proctriage

import (
	"context"

	"github.com/jdefrancesco/macscope/internal/codesign"
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/memory"
	"github.com/jdefrancesco/macscope/internal/process"
)

type Report struct {
	Process   process.Info        `json:"process"`
	Signing   codesign.Inspection `json:"signing"`
	Findings  []Finding           `json:"findings,omitempty"`
	NextSteps []string            `json:"next_steps,omitempty"`
	Maps      *memory.Report      `json:"maps,omitempty"`
	Tree      *process.Tree       `json:"tree,omitempty"`
	Warnings  []string            `json:"warnings,omitempty"`
}

type Options struct {
	Query  string
	Maps   bool
	Tree   bool
	Full   bool
	Runner process.CommandRunner
}

type Finding struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
	Source     string   `json:"source"`
}

func Analyze(ctx context.Context, query string, runner collect.Runner) (Report, error) {
	return AnalyzeWithOptions(ctx, Options{Query: query, Runner: runner})
}

func AnalyzeWithOptions(ctx context.Context, opts Options) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	runner := opts.Runner
	if runner == nil {
		native, err := collect.MacOSRunner()
		if err != nil {
			return Report{}, err
		}
		runner = native
	}
	info, err := process.Lookup(ctx, opts.Query, runner)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		Process: info,
		NextSteps: []string{
			"use macscope attach <pid> for debugger attach checks",
			"use macscope proc --maps --tree <pid> for runtime regions and ancestry",
		},
	}
	identity, identityErr := process.Identity(ctx, info.PID, runner)
	if identityErr == nil {
		info.Path = identity.Path
		info.Comm = identity.Path
		info.Name = identity.Name
		info.StartedAt = identity.StartedAt
		info.PPID = identity.PPID
		report.Process = info
	} else {
		report.Warnings = append(report.Warnings, identityErr.Error())
		// Legacy combined ps rows cannot unambiguously delimit paths with spaces.
		// Do not sign-check a potentially truncated executable path.
		info.Path = ""
		report.Process.Path = ""
	}
	if opts.Tree {
		tree, err := process.SnapshotTree(ctx, info.PID, runner)
		if err != nil {
			report.Warnings = append(report.Warnings, err.Error())
			report.Tree = &process.Tree{Status: "unavailable", Ancestors: []process.Node{}, Children: []process.Node{}, Warnings: []string{err.Error()}}
		} else {
			report.Tree = &tree
		}
	}
	if opts.Maps {
		maps, err := memory.Collect(ctx, info.PID, opts.Full, runner)
		if err != nil {
			return Report{}, err
		}
		report.Maps = &maps
	}
	if info.Path != "" {
		report.Signing = codesign.InspectPath(ctx, info.Path, runner)
		report.Findings = signingFindings(report.Signing)
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	if identityErr == nil && (opts.Maps || opts.Tree) {
		after, err := process.Identity(ctx, info.PID, runner)
		if err != nil {
			report.Warnings = append(report.Warnings, "target recheck: "+err.Error())
		} else if after.StartedAt != identity.StartedAt || after.Path != identity.Path {
			report.Warnings = append(report.Warnings, "target identity changed during collection; maps and ancestry may span different executions")
		}
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
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
