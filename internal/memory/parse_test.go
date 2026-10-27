package memory

import (
	"os"
	"strings"
	"testing"
)

func TestParseVMMap(t *testing.T) {
	data, err := os.ReadFile("../../testdata/memory/vmmap.txt")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Parse(string(data), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Regions) != 6 || len(report.Images) != 2 || report.Regions[0].SizeBytes != 16384 || report.Regions[0].Protection != "r-x" || report.Regions[0].MaxProtection != "rwx" || report.Regions[5].Path != "/Users/alice/data file.bin" {
		t.Fatalf("report=%#v", report)
	}
	if report.Images[0].Path != "/Applications/Example App.app/Contents/MacOS/Example App" || len(report.Images[0].RegionStarts) != 2 {
		t.Fatalf("images=%#v", report.Images)
	}
	for _, input := range []string{"", "vmmap: permission denied", "Process: Other [99]\n" + strings.Join(strings.Split(string(data), "\n")[2:], "\n"), strings.Replace(string(data), "100000000-100004000", "100004000-100000000", 1)} {
		if _, err := Parse(input, 42); err == nil {
			t.Errorf("accepted invalid vmmap output %q", input)
		}
	}
	partial := string(data) + "__DATA 500000000-500004000 [16K] rw?/rwx SM=COW /tmp/test\n"
	got, err := Parse(partial, 42)
	if err != nil || got.Status != "partial" || len(got.Warnings) == 0 {
		t.Fatalf("partial=%#v err=%v", got, err)
	}
}
