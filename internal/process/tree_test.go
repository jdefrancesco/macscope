package process

import (
	"os"
	"strings"
	"testing"
)

func TestParseSnapshotAndTree(t *testing.T) {
	data, err := os.ReadFile("../../testdata/process/tree.txt")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := ParseSnapshot(string(data))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := BuildTree(nodes, 42)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Target.Path != "/Applications/Example App.app/Contents/MacOS/Example App" || tree.Target.Name != "Example App" || tree.Target.StartedAt != "Mon Oct 26 09:00:00 2026" {
		t.Fatalf("target = %#v", tree.Target)
	}
	if len(tree.Ancestors) != 2 || tree.Ancestors[0].PID != 20 || tree.Ancestors[1].PID != 1 || len(tree.Children) != 2 || tree.Children[0].PID != 77 {
		t.Fatalf("tree = %#v", tree)
	}
	for _, input := range []string{"", "bad", "42 1 bad date /bin/test", "-1 0 Mon Oct 26 08:00:00 2026 /bin/test", string(data) + strings.Split(string(data), "\n")[0]} {
		if _, err := ParseSnapshot(input); err == nil {
			t.Errorf("accepted invalid snapshot %q", input)
		}
	}
}

func TestTreeMissingAndCycles(t *testing.T) {
	for _, tt := range []struct {
		name    string
		nodes   []Node
		pid     int
		bad     bool
		warning bool
	}{
		{"absent target", []Node{{PID: 1}}, 42, true, false},
		{"missing parent", []Node{{PID: 42, PPID: 20}}, 42, false, true},
		{"cycle", []Node{{PID: 42, PPID: 20}, {PID: 20, PPID: 42}}, 42, false, true},
		{"root", []Node{{PID: 1, PPID: 0}}, 1, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildTree(tt.nodes, tt.pid)
			if (err != nil) != tt.bad || !tt.bad && (len(got.Warnings) > 0) != tt.warning {
				t.Fatalf("tree=%#v err=%v", got, err)
			}
		})
	}
}
