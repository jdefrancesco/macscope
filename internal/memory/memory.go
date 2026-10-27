// Package memory parses read-only vmmap snapshots; it never reads payload bytes.
package memory

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/process"
)

type Region struct {
	Type          string `json:"type"`
	Start         string `json:"start"`
	End           string `json:"end"`
	SizeBytes     uint64 `json:"size_bytes"`
	Protection    string `json:"protection"`
	MaxProtection string `json:"max_protection"`
	Sharing       string `json:"sharing"`
	Path          string `json:"path,omitempty"`
	Detail        string `json:"detail,omitempty"`
}
type Image struct {
	Path         string   `json:"path"`
	RegionStarts []string `json:"region_starts"`
}
type Report struct {
	PID        int             `json:"pid"`
	Source     string          `json:"source"`
	Status     string          `json:"status"`
	CapturedAt time.Time       `json:"captured_at"`
	Regions    []Region        `json:"regions"`
	Images     []Image         `json:"images"`
	Warnings   []string        `json:"warnings,omitempty"`
	Raw        *collect.Result `json:"raw,omitempty"`
}

var regionRow = regexp.MustCompile(`^\s*(.*?)\s+([0-9A-Fa-f]+)-([0-9A-Fa-f]+)\s+\[[^\]]*\]\s+([rwx-]{3})/([rwx-]{3})\s+SM=([A-Z/]+)\s*(.*)$`)
var processHeader = regexp.MustCompile(`(?m)^Process:\s+.*\[([0-9]+)\]\s*$`)
var addressRange = regexp.MustCompile(`[0-9A-Fa-f]{4,}-[0-9A-Fa-f]{4,}`)

func Parse(data string, pid int) (Report, error) {
	report := Report{PID: pid, Status: "ok", Regions: []Region{}, Images: []Image{}}
	header := processHeader.FindStringSubmatch(data)
	if len(header) != 2 || header[1] != strconv.Itoa(pid) {
		return report, fmt.Errorf("vmmap returned no matching process header for PID %d", pid)
	}
	images := make(map[string][]string)
	for _, line := range strings.Split(data, "\n") {
		m := regionRow.FindStringSubmatch(line)
		if m == nil {
			if addressRange.MatchString(line) && strings.Contains(line, "SM=") {
				report.Warnings = append(report.Warnings, "unparsed vmmap region: "+strings.TrimSpace(line))
			}
			continue
		}
		start, e1 := strconv.ParseUint(m[2], 16, 64)
		end, e2 := strconv.ParseUint(m[3], 16, 64)
		if e1 != nil || e2 != nil || end <= start {
			return report, fmt.Errorf("invalid vmmap address range %s-%s", m[2], m[3])
		}
		region := Region{Type: strings.TrimSpace(m[1]), Start: "0x" + strings.ToLower(m[2]), End: "0x" + strings.ToLower(m[3]), SizeBytes: end - start, Protection: m[4], MaxProtection: m[5], Sharing: m[6], Detail: strings.TrimSpace(m[7])}
		if strings.HasPrefix(region.Detail, "/") {
			region.Path = region.Detail
			region.Detail = ""
		}
		report.Regions = append(report.Regions, region)
		if region.Path != "" && strings.HasPrefix(region.Type, "__") {
			images[region.Path] = append(images[region.Path], region.Start)
		}
	}
	if len(report.Regions) == 0 {
		return report, fmt.Errorf("vmmap returned no recognized regions for PID %d; check task access and output format", pid)
	}
	for path, starts := range images {
		report.Images = append(report.Images, Image{Path: path, RegionStarts: starts})
	}
	sort.Slice(report.Images, func(i, j int) bool { return report.Images[i].Path < report.Images[j].Path })
	if len(report.Warnings) > 0 {
		report.Status = "partial"
	}
	return report, nil
}

func Collect(ctx context.Context, pid int, full bool, runner process.CommandRunner) (Report, error) {
	if pid <= 0 {
		return Report{}, fmt.Errorf("invalid PID: %d", pid)
	}
	if runner == nil {
		native, err := collect.MacOSRunner()
		if err != nil {
			return Report{}, err
		}
		runner = native
	}
	result, runErr := runner.Run(ctx, "/usr/bin/vmmap", "-w", strconv.Itoa(pid))
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	report, parseErr := Parse(result.Stdout, pid)
	report.Source = "/usr/bin/vmmap -w " + strconv.Itoa(pid)
	report.CapturedAt = time.Now().UTC()
	if parseErr != nil {
		report.Status = "unavailable"
		report.Regions = []Region{}
		report.Images = []Image{}
		report.Warnings = append(report.Warnings, parseErr.Error())
	}
	if runErr != nil {
		report.Warnings = append(report.Warnings, "vmmap collection: "+runErr.Error()+"; task access may be restricted or the process may have exited")
	}
	if detail := strings.TrimSpace(result.Stderr); detail != "" {
		report.Warnings = append(report.Warnings, detail)
	}
	if report.Status == "ok" && len(report.Warnings) > 0 {
		report.Status = "partial"
	}
	if full {
		report.Raw = &result
	}
	return report, nil
}
