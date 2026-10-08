package launchd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jdefrancesco/macscope/internal/collect"
)

// Inventory describes installed configuration, not launchctl runtime state.
type Inventory struct {
	Kind        string   `json:"kind"`
	Directories []string `json:"directories"`
	Jobs        []Job    `json:"jobs"`
	Errors      []string `json:"errors,omitempty"`
}

func InventoryDirs(kind string) []string {
	if kind == "daemons" {
		return []string{"/System/Library/LaunchDaemons", "/Library/LaunchDaemons"}
	}
	dirs := []string{"/System/Library/LaunchAgents", "/Library/LaunchAgents"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, "Library", "LaunchAgents"))
	}
	return dirs
}

func List(ctx context.Context, kind string, dirs []string) (Inventory, error) {
	if kind != "agents" && kind != "daemons" {
		return Inventory{}, fmt.Errorf("unknown launchd inventory kind %q", kind)
	}
	var runner collect.Runner
	if len(dirs) == 0 {
		var err error
		runner, err = collect.MacOSRunner()
		if err != nil {
			return Inventory{}, err
		}
		dirs = InventoryDirs(kind)
	}
	report := Inventory{Kind: kind, Directories: dirs, Jobs: []Job{}}
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return Inventory{}, err
		}
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			report.Errors = append(report.Errors, dir+": "+err.Error())
			continue
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return Inventory{}, err
			}
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".plist") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err == nil && bytes.HasPrefix(data, []byte("bplist")) {
				if _, platformErr := collect.MacOSRunner(); platformErr != nil {
					err = platformErr
				} else {
					var result collect.Result
					result, err = runner.Run(ctx, "/usr/bin/plutil", "-convert", "xml1", "-o", "-", "--", path)
					if err == nil {
						data = []byte(result.Stdout)
					}
				}
			}
			if err != nil {
				report.Errors = append(report.Errors, path+": "+err.Error())
				continue
			}
			job, err := ParsePlist(path, data)
			if err != nil {
				report.Errors = append(report.Errors, path+": "+err.Error())
				continue
			}
			report.Jobs = append(report.Jobs, job)
		}
	}
	sort.Slice(report.Jobs, func(i, j int) bool { return report.Jobs[i].Path < report.Jobs[j].Path })
	return report, nil
}
