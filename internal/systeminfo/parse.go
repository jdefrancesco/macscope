package systeminfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Report struct {
	Hardware Hardware `json:"hardware"`
	OS       OS       `json:"os"`
}

type Hardware struct {
	ModelName             string     `json:"model_name,omitempty"`
	ModelIdentifier       string     `json:"model_identifier"`
	Processor             string     `json:"processor,omitempty"`
	ProcessorSpeed        string     `json:"processor_speed,omitempty"`
	MemoryBytes           int64      `json:"memory_bytes"`
	PhysicalCores         int        `json:"physical_cores"`
	LogicalCores          int        `json:"logical_cores"`
	Architecture          string     `json:"architecture"`
	ExecutionArchitecture string     `json:"execution_architecture,omitempty"`
	Graphics              []Graphics `json:"graphics"`
}

type Graphics struct {
	Model string `json:"model"`
	Cores int    `json:"cores,omitempty"`
	VRAM  string `json:"vram,omitempty"`
}

type OS struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Build   string `json:"build"`
}

// ParseHardware reads only specification fields, excluding device identifiers.
func ParseHardware(input string) (Hardware, error) {
	var data struct {
		Hardware []struct {
			ModelName       string `json:"machine_name"`
			ModelIdentifier string `json:"machine_model"`
			Chip            string `json:"chip_type"`
			CPU             string `json:"cpu_type"`
			Speed           string `json:"current_processor_speed"`
		} `json:"SPHardwareDataType"`
		Graphics []struct {
			Name  string `json:"_name"`
			Model string `json:"sppci_model"`
			Cores string `json:"sppci_cores"`
			VRAM  string `json:"spdisplays_vram"`
		} `json:"SPDisplaysDataType"`
	}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return Hardware{}, fmt.Errorf("parse system_profiler JSON: %w", err)
	}
	if len(data.Hardware) == 0 || data.Hardware[0].ModelIdentifier == "" {
		return Hardware{}, errors.New("system_profiler returned no hardware model")
	}
	hw := data.Hardware[0]
	processor := hw.Chip
	if processor == "" {
		processor = hw.CPU
	}
	report := Hardware{
		ModelName:       hw.ModelName,
		ModelIdentifier: hw.ModelIdentifier,
		Processor:       processor,
		ProcessorSpeed:  hw.Speed,
		Graphics:        make([]Graphics, 0, len(data.Graphics)),
	}
	// Apple Silicon chip metadata identifies native hardware even under Rosetta.
	if strings.HasPrefix(hw.Chip, "Apple ") {
		report.Architecture = "arm64"
	}
	for _, gpu := range data.Graphics {
		model := gpu.Model
		if model == "" {
			model = gpu.Name
		}
		if model == "" {
			continue
		}
		cores, err := strconv.Atoi(gpu.Cores)
		if err != nil || cores < 0 {
			cores = 0
		}
		report.Graphics = append(report.Graphics, Graphics{Model: model, Cores: cores, VRAM: gpu.VRAM})
	}
	return report, nil
}

func parseResources(input string) (Hardware, error) {
	values := parseKeyValues(input)
	memory, err := strconv.ParseInt(values["hw.memsize"], 10, 64)
	if err != nil || memory <= 0 {
		return Hardware{}, errors.New("sysctl returned invalid or missing hw.memsize")
	}
	physical, err := strconv.Atoi(values["hw.physicalcpu"])
	if err != nil || physical <= 0 {
		return Hardware{}, errors.New("sysctl returned invalid or missing hw.physicalcpu")
	}
	logical, err := strconv.Atoi(values["hw.logicalcpu"])
	if err != nil || logical <= 0 {
		return Hardware{}, errors.New("sysctl returned invalid or missing hw.logicalcpu")
	}
	architecture := values["hw.machine"]
	if architecture == "" {
		return Hardware{}, errors.New("sysctl returned no hw.machine architecture")
	}
	return Hardware{MemoryBytes: memory, PhysicalCores: physical, LogicalCores: logical, Architecture: architecture}, nil
}

// ParseOS parses the labelled output of sw_vers.
func ParseOS(input string) (OS, error) {
	values := parseKeyValues(input)
	os := OS{Name: values["ProductName"], Version: values["ProductVersion"], Build: values["BuildVersion"]}
	if os.Name == "" || os.Version == "" || os.Build == "" {
		return OS{}, errors.New("sw_vers returned incomplete OS version information")
	}
	return os, nil
}

func parseKeyValues(input string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(input, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values
}
