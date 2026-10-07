package diskspace

import (
	"os"
	"testing"
)

func dfFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/diskspace/df.txt")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseDF(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		rows        int
		wantErr     bool
	}{
		{"APFS and paths with spaces", dfFixture(t), 6, false},
		{"full volume", "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/disk1 100 110 -10 110% /", 1, false},
		{"empty", "", 0, true},
		{"header only", "Filesystem 1024-blocks Used Available Capacity Mounted on\n", 0, true},
		{"malformed row", "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/disk1 invalid", 0, true},
		{"overflow", "/dev/disk1 9223372036854775807 0 0 0% /", 0, true},
		{"negative total", "/dev/disk1 -1 0 0 0% /", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDF(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseDF error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != tt.rows {
				t.Fatalf("row count = %d, want %d", len(got), tt.rows)
			}
			if tt.rows == 6 {
				if got[0].TotalBytes != 1024000000 || got[0].UsedBytes != 102400000 || got[0].AvailableBytes != 204800000 || got[0].CapacityPercent != 34 {
					t.Fatalf("incorrect block conversion or APFS capacity: %#v", got[0])
				}
				if got[4].MountPoint != "/Volumes/Backup  Drive" || got[5].Filesystem != "devices -- file:///Users/alice/Library/Containers/" {
					t.Fatalf("spaces not preserved: %#v", got)
				}
			} else if got[0].AvailableBytes != -10240 {
				t.Fatalf("available bytes = %d, want -10240", got[0].AvailableBytes)
			}
		})
	}
}
