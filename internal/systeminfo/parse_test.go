package systeminfo

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/systeminfo/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseHardware(t *testing.T) {
	tests := []struct {
		name, input, model, processor, speed string
		graphics, gpuCores                   int
		wantErr                              bool
	}{
		{name: "Apple Silicon", input: fixture(t, "apple_silicon.json"), model: "Mac16,8", processor: "Apple M4 Pro", graphics: 1, gpuCores: 20},
		{name: "Intel with multiple GPUs", input: fixture(t, "intel.json"), model: "iMac19,1", processor: "Intel Core i9", speed: "3.6 GHz", graphics: 2},
		{name: "optional graphics absent", input: `{"SPHardwareDataType":[{"machine_model":"Mac14,2"}]}`, model: "Mac14,2"},
		{name: "malformed JSON", input: `{`, wantErr: true},
		{name: "missing hardware", input: `{}`, wantErr: true},
		{name: "empty hardware", input: `{"SPHardwareDataType":[]}`, wantErr: true},
		{name: "empty record", input: `{"SPHardwareDataType":[{}]}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHardware(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseHardware error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.ModelIdentifier != tt.model || got.Processor != tt.processor || got.ProcessorSpeed != tt.speed || len(got.Graphics) != tt.graphics {
				t.Fatalf("unexpected hardware: %#v", got)
			}
			if tt.gpuCores > 0 && got.Graphics[0].Cores != tt.gpuCores {
				t.Fatalf("GPU cores = %d, want %d", got.Graphics[0].Cores, tt.gpuCores)
			}
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "PRIVATE") {
				t.Fatalf("hardware exposed identifiers: %s", data)
			}
		})
	}
}

func TestParseResources(t *testing.T) {
	valid := fixture(t, "sysctl.txt")
	tests := []struct {
		name, input string
		wantErr     bool
	}{
		{"valid", valid, false},
		{"missing memory", strings.ReplaceAll(valid, "hw.memsize: 51539607552\n", ""), true},
		{"invalid memory", strings.ReplaceAll(valid, "51539607552", "unknown"), true},
		{"negative memory", strings.ReplaceAll(valid, "51539607552", "-1"), true},
		{"zero cores", strings.ReplaceAll(valid, "hw.physicalcpu: 14", "hw.physicalcpu: 0"), true},
		{"missing architecture", strings.ReplaceAll(valid, "hw.machine: arm64\n", ""), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseResources(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseResources error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (got.MemoryBytes != 51539607552 || got.PhysicalCores != 14 || got.LogicalCores != 14 || got.Architecture != "arm64") {
				t.Fatalf("unexpected resources: %#v", got)
			}
		})
	}
}

func TestParseGPUCoreCount(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  int
	}{
		{"20", 20}, {"", 0}, {"unknown", 0}, {"-1", 0}, {"9999999999999999999999999999999999", 0},
	} {
		t.Run(tt.input, func(t *testing.T) {
			input := strings.ReplaceAll(fixture(t, "apple_silicon.json"), `"sppci_cores": "20"`, `"sppci_cores": "`+tt.input+`"`)
			got, err := ParseHardware(input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Graphics[0].Cores != tt.want {
				t.Fatalf("cores = %d, want %d", got.Graphics[0].Cores, tt.want)
			}
		})
	}
}

func TestParseOS(t *testing.T) {
	valid := fixture(t, "sw_vers.txt")
	for _, tt := range []struct {
		name, input string
		wantErr     bool
	}{
		{"valid", valid, false},
		{"empty", "", true},
		{"missing build", "ProductName: macOS\nProductVersion: 15.5", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOS(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseOS error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (got.Name != "macOS" || got.Version != "15.5" || got.Build != "24F74") {
				t.Fatalf("unexpected OS: %#v", got)
			}
		})
	}
}
