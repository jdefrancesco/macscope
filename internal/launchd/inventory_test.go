package launchd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestList(t *testing.T) {
	for _, kind := range []string{"agents", "daemons"} {
		t.Run(kind, func(t *testing.T) {
			report, err := List(context.Background(), kind, []string{filepath.Join("..", "..", "testdata", "launchd")})
			if err != nil || len(report.Jobs) == 0 || len(report.Errors) != 0 {
				t.Fatalf("report=%+v err=%v", report, err)
			}
		})
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.plist"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := List(context.Background(), "agents", []string{dir})
	if err != nil || len(report.Errors) != 1 || report.Jobs == nil {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := List(ctx, "agents", []string{dir}); err == nil {
		t.Fatal("expected cancellation")
	}
}

func TestStartupSettings(t *testing.T) {
	for _, tt := range []struct {
		name, keys          string
		run, keep, disabled bool
		detail              string
	}{
		{name: "on demand"},
		{name: "run at load", keys: "<key>RunAtLoad</key><true/>", run: true},
		{name: "keep alive", keys: "<key>KeepAlive</key><true/>", keep: true},
		{name: "conditional", keys: "<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>", keep: true, detail: "dictionary"},
		{name: "disabled", keys: "<key>Disabled</key><true/>", disabled: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			job, err := ParsePlist("fixture.plist", []byte("<plist><dict><key>Label</key><string>example</string>"+tt.keys+"</dict></plist>"))
			if err != nil || job.RunAtLoad != tt.run || job.KeepAlive != tt.keep || job.Disabled != tt.disabled || job.KeepAliveDetail != tt.detail {
				t.Fatalf("job=%+v err=%v", job, err)
			}
		})
	}
}
