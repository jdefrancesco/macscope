package systeminfo

import (
	"context"
	"fmt"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) (collect.Result, error)
}

// Analyze collects system specifications without serial numbers or hostnames.
func Analyze(ctx context.Context, runner CommandRunner) (Report, error) {
	if runner == nil {
		native, err := collect.MacOSRunner()
		if err != nil {
			return Report{}, err
		}
		runner = native
	}
	result, err := runner.Run(ctx, "/usr/sbin/system_profiler", "-json", "-detailLevel", "mini", "SPHardwareDataType", "SPDisplaysDataType")
	if err != nil {
		return Report{}, fmt.Errorf("collect hardware with system_profiler: %w", err)
	}
	hardware, err := ParseHardware(result.Stdout)
	if err != nil {
		return Report{}, err
	}
	result, err = runner.Run(ctx, "/usr/sbin/sysctl", "hw.memsize", "hw.physicalcpu", "hw.logicalcpu", "hw.machine")
	if err != nil {
		return Report{}, fmt.Errorf("collect CPU and memory with sysctl: %w", err)
	}
	resources, err := parseResources(result.Stdout)
	if err != nil {
		return Report{}, err
	}
	hardware.MemoryBytes = resources.MemoryBytes
	hardware.PhysicalCores = resources.PhysicalCores
	hardware.LogicalCores = resources.LogicalCores
	if hardware.Architecture == "" {
		hardware.Architecture = resources.Architecture
	} else if hardware.Architecture != resources.Architecture {
		hardware.ExecutionArchitecture = resources.Architecture
	}
	result, err = runner.Run(ctx, "/usr/bin/sw_vers")
	if err != nil {
		return Report{}, fmt.Errorf("collect macOS version with sw_vers: %w", err)
	}
	os, err := ParseOS(result.Stdout)
	if err != nil {
		return Report{}, err
	}
	return Report{Hardware: hardware, OS: os}, nil
}
