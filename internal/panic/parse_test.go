package paniclog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const watchdogFixture = `panic(cpu 2 caller 0xfffffe001234abcd): watchdog timeout: no checkins from watchdogd in 92 seconds
bootsessionuuid: 01234567-89AB-CDEF-0123-456789ABCDEF
osversion: 23G80,
Current Phase = "IOKit Boot"
PanicMedic Boot path active
AppleDisplayCrossbar event near panic window
Thunderbolt link reset
Saved type '210' report panic-full at /Library/Logs/DiagnosticReports/panic-full-2026-05-19-142007.0003.panic
SOCD report detected
`

func TestParseWatchdogPanic(t *testing.T) {
	report := Parse("/tmp/example.panic", watchdogFixture)

	if report.PanicType != TypeWatchdogTimeout {
		t.Fatalf("PanicType = %q, want %q", report.PanicType, TypeWatchdogTimeout)
	}
	if report.CPU == nil || *report.CPU != 2 {
		t.Fatalf("CPU = %#v, want 2", report.CPU)
	}
	if report.WatchdogTimeoutSec == nil || *report.WatchdogTimeoutSec != 92 {
		t.Fatalf("WatchdogTimeoutSec = %#v, want 92", report.WatchdogTimeoutSec)
	}
	if report.CallerAddress != "0xfffffe001234abcd" {
		t.Fatalf("CallerAddress = %q", report.CallerAddress)
	}
	if report.OSVersion != "23G80" {
		t.Fatalf("OSVersion = %q, want 23G80", report.OSVersion)
	}
	if report.BootSessionUUID != "01234567-89AB-CDEF-0123-456789ABCDEF" {
		t.Fatalf("BootSessionUUID = %q", report.BootSessionUUID)
	}
	if report.CurrentPhase != "IOKit Boot" {
		t.Fatalf("CurrentPhase = %q", report.CurrentPhase)
	}
	if report.SavedReportPath == "" {
		t.Fatal("SavedReportPath = empty")
	}
	if report.SOCDPresent == nil || !*report.SOCDPresent {
		t.Fatalf("SOCDPresent = %#v, want true", report.SOCDPresent)
	}
	if !hasCause(report, "BOOT_IOKIT_STALL") {
		t.Fatalf("SuspectedCauses = %#v, want BOOT_IOKIT_STALL", report.SuspectedCauses)
	}
	if !hasCause(report, "EXTERNAL_DISPLAY_OR_DOCK") {
		t.Fatalf("SuspectedCauses = %#v, want EXTERNAL_DISPLAY_OR_DOCK", report.SuspectedCauses)
	}
}

func TestParseKernelPanicWithoutWatchdog(t *testing.T) {
	report := Parse("", `panic(cpu 0 caller 0xfffffe0011112222): unexpected kernel trap`)

	if report.PanicType != TypeKernelPanic {
		t.Fatalf("PanicType = %q, want %q", report.PanicType, TypeKernelPanic)
	}
	if len(report.SuspectedCauses) != 0 {
		t.Fatalf("SuspectedCauses = %#v, want none", report.SuspectedCauses)
	}
}

func TestLatestFileAndFindFilesSince(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.panic")
	newPath := filepath.Join(dir, "panic-full-new.panic")
	ignoredPath := filepath.Join(dir, "note.txt")

	mustWrite(t, oldPath, "old")
	mustWrite(t, newPath, "new")
	mustWrite(t, ignoredPath, "ignored")

	oldTime := time.Now().Add(-3 * time.Hour)
	newTime := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	latest, err := LatestFile([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if latest.Path != newPath {
		t.Fatalf("LatestFile path = %q, want %q", latest.Path, newPath)
	}

	files, err := FindFilesSince([]string{dir}, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != newPath {
		t.Fatalf("FindFilesSince = %#v, want only new panic", files)
	}
}

func hasCause(report Report, category string) bool {
	for _, cause := range report.SuspectedCauses {
		if cause.Category == category {
			return true
		}
	}
	return false
}

func mustWrite(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}
