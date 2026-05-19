package launchd

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Report struct {
	Directories []string  `json:"directories"`
	Jobs        []Job     `json:"jobs"`
	Findings    []Finding `json:"findings,omitempty"`
	Errors      []string  `json:"errors,omitempty"`
}

type Finding struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Confidence float64  `json:"confidence"`
	Score      int      `json:"score"`
	JobLabel   string   `json:"job_label,omitempty"`
	JobPath    string   `json:"job_path"`
	Program    string   `json:"program,omitempty"`
	Evidence   []string `json:"evidence"`
	Source     string   `json:"source"`
}

func DefaultDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return []string{"/Library/LaunchAgents", "/Library/LaunchDaemons"}
	}
	return []string{
		"/Library/LaunchAgents",
		"/Library/LaunchDaemons",
		filepath.Join(home, "Library", "LaunchAgents"),
	}
}

func AnalyzeDirs(dirs []string) Report {
	if len(dirs) == 0 {
		dirs = DefaultDirs()
	}

	report := Report{
		Directories: dirs,
	}

	for _, dir := range dirs {
		jobs, errs := ReadJobs(dir)
		report.Jobs = append(report.Jobs, jobs...)
		report.Errors = append(report.Errors, errs...)
	}

	sort.Slice(report.Jobs, func(i, j int) bool {
		return report.Jobs[i].Path < report.Jobs[j].Path
	})

	for _, job := range report.Jobs {
		report.Findings = append(report.Findings, ScoreJob(job)...)
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Score == report.Findings[j].Score {
			return report.Findings[i].JobPath < report.Findings[j].JobPath
		}
		return report.Findings[i].Score > report.Findings[j].Score
	})

	return report
}

func ReadJobs(dir string) ([]Job, []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return nil, nil
		}
		return nil, []string{dir + ": " + err.Error()}
	}

	var jobs []Job
	var errs []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".plist") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, path+": "+err.Error())
			continue
		}
		job, err := ParsePlist(path, data)
		if err != nil {
			errs = append(errs, path+": "+err.Error())
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, errs
}

func ScoreJob(job Job) []Finding {
	program := EffectiveProgram(job)
	var findings []Finding

	if writablePath := userWritablePath(job); writablePath != "" {
		findings = append(findings, finding(job, "USER_WRITABLE_PERSISTENCE", "high", 0.86, 30, []string{
			"program path is under a user-writable or transient directory",
			"path=" + writablePath,
		}))
	}
	if usesShell(job) {
		findings = append(findings, finding(job, "SHELL_LAUNCHD_JOB", "medium", 0.72, 15, []string{
			"launch item invokes a shell interpreter",
			"arguments=" + strings.Join(job.ProgramArguments, " "),
		}))
	}
	if usesNetworkDownloader(job) {
		findings = append(findings, finding(job, "NETWORK_DOWNLOADER_PERSISTENCE", "medium", 0.74, 20, []string{
			"launch item arguments include a network downloader or URL",
			"arguments=" + strings.Join(job.ProgramArguments, " "),
		}))
	}
	if job.RunAtLoad && (program != "" || len(job.ProgramArguments) > 0) {
		findings = append(findings, finding(job, "RUN_AT_LOAD", "low", 0.65, 5, []string{
			"RunAtLoad is enabled",
		}))
	}
	if job.KeepAlive {
		evidence := []string{"KeepAlive is enabled"}
		if job.KeepAliveDetail != "" {
			evidence = append(evidence, "KeepAlive="+job.KeepAliveDetail)
		}
		findings = append(findings, finding(job, "KEEPALIVE_ENABLED", "low", 0.62, 5, evidence))
	}

	return findings
}

func EffectiveProgram(job Job) string {
	if job.Program != "" {
		return job.Program
	}
	for _, arg := range job.ProgramArguments {
		if strings.TrimSpace(arg) != "" {
			return arg
		}
	}
	return ""
}

func finding(job Job, category, severity string, confidence float64, score int, evidence []string) Finding {
	return Finding{
		Category:   category,
		Severity:   severity,
		Confidence: confidence,
		Score:      score,
		JobLabel:   job.Label,
		JobPath:    job.Path,
		Program:    EffectiveProgram(job),
		Evidence:   evidence,
		Source:     "launchd plist",
	}
}

func isUserWritableOrTransient(program string) bool {
	program = filepath.Clean(program)
	home, _ := os.UserHomeDir()
	return strings.HasPrefix(program, "/tmp/") ||
		strings.HasPrefix(program, "/private/tmp/") ||
		strings.HasPrefix(program, "/var/folders/") ||
		strings.HasPrefix(program, "/Users/") ||
		(home != "" && strings.HasPrefix(program, filepath.Clean(home)+string(os.PathSeparator)))
}

func userWritablePath(job Job) string {
	candidates := append([]string{job.Program}, job.ProgramArguments...)
	for _, candidate := range candidates {
		for _, field := range strings.Fields(candidate) {
			field = strings.Trim(field, `"'`)
			if strings.HasPrefix(field, "file://") {
				field = strings.TrimPrefix(field, "file://")
			}
			if strings.HasPrefix(field, "/") && isUserWritableOrTransient(field) {
				return field
			}
		}
	}
	return ""
}

func usesShell(job Job) bool {
	fields := append([]string{job.Program}, job.ProgramArguments...)
	for _, value := range fields {
		base := filepath.Base(value)
		switch base {
		case "sh", "bash", "zsh", "osascript", "python", "python3", "perl", "ruby":
			return true
		}
	}
	args := strings.ToLower(strings.Join(job.ProgramArguments, " "))
	return strings.Contains(args, " sh -c ") ||
		strings.Contains(args, " bash -c ") ||
		strings.Contains(args, " zsh -c ")
}

func usesNetworkDownloader(job Job) bool {
	args := strings.ToLower(strings.Join(job.ProgramArguments, " "))
	return strings.Contains(args, "curl") ||
		strings.Contains(args, "wget") ||
		strings.Contains(args, "http://") ||
		strings.Contains(args, "https://")
}
