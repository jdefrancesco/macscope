package diskspace

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) (collect.Result, error)
}

type Options struct {
	Path   string
	All    bool
	Runner CommandRunner
}

// Analyze reports local disks, or the mounted filesystem containing a path.
func Analyze(ctx context.Context, opts Options) (Report, error) {
	runner := opts.Runner
	if runner == nil {
		native, err := collect.MacOSRunner()
		if err != nil {
			return Report{}, err
		}
		runner = native
	}
	args := []string{"-k", "-P", "-I"}
	path := ""
	if opts.Path != "" {
		var err error
		path, err = filepath.Abs(opts.Path)
		if err != nil {
			return Report{}, fmt.Errorf("resolve disk path: %w", err)
		}
		args = append(args, path)
	} else {
		args = append(args, "-l")
	}
	result, err := runner.Run(ctx, "/bin/df", args...)
	if err != nil {
		detail := strings.TrimSpace(result.Stderr)
		if detail != "" {
			return Report{}, fmt.Errorf("collect disk space: %w: %s", err, detail)
		}
		return Report{}, fmt.Errorf("collect disk space: %w", err)
	}
	volumes, err := ParseDF(result.Stdout)
	if err != nil {
		return Report{}, err
	}
	if !opts.All && path == "" {
		visible := make([]Volume, 0, len(volumes))
		for _, volume := range volumes {
			if !strings.HasPrefix(volume.Filesystem, "/dev/") {
				continue
			}
			if strings.HasPrefix(volume.MountPoint, "/System/Volumes/") && volume.MountPoint != "/System/Volumes/Data" {
				continue
			}
			visible = append(visible, volume)
		}
		volumes = visible
	}
	return Report{RequestedPath: path, Volumes: volumes}, nil
}
