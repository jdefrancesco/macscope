package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/jdefrancesco/macscope/internal/output"
	"github.com/jdefrancesco/macscope/internal/systeminfo"
)

type specsFlags struct {
	json bool
	help bool
}

func runSpecs(ctx context.Context, args []string, streams output.Streams) error {
	flags, err := parseSpecsFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printSpecsHelp(streams.Out)
		return nil
	}
	report, err := systeminfo.Analyze(ctx, nil)
	if err != nil {
		return err
	}
	if flags.json {
		return output.WriteJSON(streams.Out, report)
	}
	return renderSpecsReport(streams.Out, report)
}

func parseSpecsFlags(args []string) (specsFlags, error) {
	var flags specsFlags
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		default:
			return specsFlags{}, fmt.Errorf("unknown specs argument: %s; usage: macscope specs [--json]", arg)
		}
	}
	return flags, nil
}

func printSpecsHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope specs [--json]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "List the Mac model, processor, CPU cores, memory, graphics, and macOS version.")
	fmt.Fprintln(w, "Read-only; requires macOS, with no sudo or additional permissions.")
	fmt.Fprintln(w, "Serial numbers, hardware UUIDs, and hostnames are omitted, including in JSON.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json   Emit stable JSON with memory in bytes.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Native tools: system_profiler, sysctl, sw_vers.")
	fmt.Fprintln(w, "Graphics fields depend on what system_profiler exposes.")
	fmt.Fprintln(w, "Execution architecture is listed separately when it differs from native hardware.")
}

func renderSpecsReport(w io.Writer, report systeminfo.Report) error {
	tw := output.NewTextWriter(w)
	if err := tw.Section("System Specs"); err != nil {
		return err
	}
	hw := report.Hardware
	model := hw.ModelIdentifier
	if hw.ModelName != "" {
		model = hw.ModelName + " (" + model + ")"
	}
	processor := fallback(hw.Processor, "unavailable")
	if hw.ProcessorSpeed != "" {
		processor += " / " + hw.ProcessorSpeed
	}
	for _, kv := range [][2]string{
		{"Model", model},
		{"Processor", processor},
		{"CPU Cores", fmt.Sprintf("%d physical / %d logical", hw.PhysicalCores, hw.LogicalCores)},
		{"Memory", formatBytes(hw.MemoryBytes)},
		{"Architecture", hw.Architecture},
		{"OS", fmt.Sprintf("%s %s (build %s)", report.OS.Name, report.OS.Version, report.OS.Build)},
	} {
		if err := tw.KeyValue(kv[0], kv[1]); err != nil {
			return err
		}
	}
	if hw.ExecutionArchitecture != "" {
		if err := tw.KeyValue("Execution Architecture", hw.ExecutionArchitecture); err != nil {
			return err
		}
	}
	if err := tw.Section("Graphics"); err != nil {
		return err
	}
	if len(hw.Graphics) == 0 {
		return tw.Bullet("graphics information unavailable")
	}
	for _, gpu := range hw.Graphics {
		line := gpu.Model
		if gpu.Cores > 0 {
			line += fmt.Sprintf(" / %d cores", gpu.Cores)
		}
		if gpu.VRAM != "" {
			line += " / VRAM " + gpu.VRAM
		}
		if err := tw.Bullet(line); err != nil {
			return err
		}
	}
	return nil
}
