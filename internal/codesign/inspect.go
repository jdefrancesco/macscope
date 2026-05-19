package codesign

import (
	"context"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) (collect.Result, error)
}

type Inspection struct {
	Details      Details      `json:"details"`
	Verification Verification `json:"verification"`
}

func InspectPath(ctx context.Context, path string, runner CommandRunner) Inspection {
	if runner == nil {
		runner = collect.Runner{}
	}

	detailsResult, _ := runner.Run(ctx, "codesign", "-dvvv", "--entitlements", ":-", path)
	verifyResult, _ := runner.Run(ctx, "codesign", "--verify", "--deep", "--strict", "--verbose=4", path)

	return Inspection{
		Details:      ParseDetails(detailsResult.Stdout, detailsResult.Stderr),
		Verification: ParseVerification(verifyResult.Stdout, verifyResult.Stderr, verifyResult.ExitCode),
	}
}
