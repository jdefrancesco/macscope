package paniclog

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var DefaultSearchDirs = []string{"/Library/Logs/DiagnosticReports"}

type FileInfo struct {
	Path    string
	ModTime time.Time
}

func LatestFile(dirs []string) (FileInfo, error) {
	files, err := FindFilesSince(dirs, time.Time{})
	if err != nil {
		return FileInfo{}, err
	}
	if len(files) == 0 {
		return FileInfo{}, errors.New("no panic reports found")
	}
	return files[0], nil
}

func FindFilesSince(dirs []string, since time.Time) ([]FileInfo, error) {
	if len(dirs) == 0 {
		dirs = DefaultSearchDirs
	}

	var files []FileInfo
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !isPanicReportName(entry.Name()) {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if !since.IsZero() && info.ModTime().Before(since) {
				continue
			}
			files = append(files, FileInfo{
				Path:    path,
				ModTime: info.ModTime(),
			})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

func ReadReport(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	return Parse(path, string(data)), nil
}

func ReadReports(paths []FileInfo) ([]Report, error) {
	reports := make([]Report, 0, len(paths))
	for _, file := range paths {
		report, err := ReadReport(file.Path)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func isPanicReportName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".panic")
}
