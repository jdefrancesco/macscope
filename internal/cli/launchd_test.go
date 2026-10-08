package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/launchd"
	"github.com/jdefrancesco/macscope/internal/output"
)

func TestLaunchInventoryCommands(t *testing.T) {
	for _, kind := range []string{"agents", "daemons"} {
		for _, format := range []string{"human", "json", "help"} {
			t.Run(kind+"/"+format, func(t *testing.T) {
				args := []string{kind, "--dir", writePersistFixture(t)}
				if format != "human" {
					args = append(args, "--"+format)
				}
				var out, errs bytes.Buffer
				if code := Run(context.Background(), args, output.Streams{Out: &out, Err: &errs}); code != 0 {
					t.Fatalf("code=%d stderr=%s", code, &errs)
				}
				if format == "json" {
					var report launchd.Inventory
					if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Kind != kind || len(report.Jobs) != 1 {
						t.Fatalf("report=%+v err=%v", report, err)
					}
				} else if !strings.Contains(out.String(), "at boot") || !strings.Contains(out.String(), "at login") {
					t.Fatalf("missing startup explanation: %s", &out)
				}
			})
		}
	}
}

func TestLaunchInventoryRedaction(t *testing.T) {
	report := launchd.Inventory{Kind: "agents", Jobs: []launchd.Job{{Label: "example", Path: "/Users/alice/Library/LaunchAgents/example.plist", Program: "/Users/alice/bin/example"}}}
	for _, full := range []bool{false, true} {
		var out bytes.Buffer
		if err := renderLaunchInventory(&out, report, full); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "alice") != full {
			t.Fatalf("full=%t output=%s", full, &out)
		}
	}
}
