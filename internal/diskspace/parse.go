package diskspace

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Report struct {
	RequestedPath string   `json:"requested_path,omitempty"`
	Volumes       []Volume `json:"volumes"`
}

type Volume struct {
	Filesystem      string `json:"filesystem"`
	MountPoint      string `json:"mount_point"`
	TotalBytes      int64  `json:"total_bytes"`
	UsedBytes       int64  `json:"used_bytes"`
	AvailableBytes  int64  `json:"available_bytes"`
	CapacityPercent int    `json:"capacity_percent"`
}

var dfRow = regexp.MustCompile(`^(.+?)\s+([0-9]+)\s+([0-9]+)\s+(-?[0-9]+)\s+([0-9]+)%\s+(.+)$`)

// ParseDF parses df output in 1024-byte blocks, preserving paths with spaces.
func ParseDF(input string) ([]Volume, error) {
	volumes := make([]Volume, 0)
	for i, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Filesystem") {
			continue
		}
		fields := dfRow.FindStringSubmatch(line)
		if fields == nil {
			return nil, fmt.Errorf("unexpected df output at line %d; expected 1024-byte block counts without inode columns", i+1)
		}
		var counts [3]int64
		for j := range counts {
			blocks, err := strconv.ParseInt(fields[j+2], 10, 64)
			if err != nil || blocks > math.MaxInt64/1024 || blocks < math.MinInt64/1024 {
				return nil, fmt.Errorf("invalid or overflowing df block count at line %d", i+1)
			}
			counts[j] = blocks * 1024
		}
		capacity, err := strconv.Atoi(fields[5])
		if err != nil {
			return nil, fmt.Errorf("invalid df capacity percentage at line %d: %w", i+1, err)
		}
		volumes = append(volumes, Volume{
			Filesystem:      fields[1],
			MountPoint:      fields[6],
			TotalBytes:      counts[0],
			UsedBytes:       counts[1],
			AvailableBytes:  counts[2],
			CapacityPercent: capacity,
		})
	}
	if len(volumes) == 0 {
		return nil, errors.New("df returned no mounted volumes")
	}
	return volumes, nil
}
