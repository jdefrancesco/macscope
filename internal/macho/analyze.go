package macho

import (
	"context"
	"crypto/sha256"
	gomacho "debug/macho"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jdefrancesco/macscope/internal/codesign"
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/gatekeeper"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) (collect.Result, error)
}

type Options struct {
	Full   bool
	Runner CommandRunner
}

type Report struct {
	InputPath            string                `json:"input_path"`
	BinaryPath           string                `json:"binary_path"`
	SizeBytes            int64                 `json:"size_bytes"`
	SHA256               string                `json:"sha256,omitempty"`
	FileType             string                `json:"file_type,omitempty"`
	Architectures        []string              `json:"architectures,omitempty"`
	LinkedLibraries      []string              `json:"linked_libraries,omitempty"`
	ExtendedAttributes   []XAttr               `json:"extended_attributes,omitempty"`
	CodeSignature        codesign.Details      `json:"code_signature"`
	CodeSignatureVerify  codesign.Verification `json:"code_signature_verify"`
	GatekeeperAssessment gatekeeper.Assessment `json:"gatekeeper_assessment"`
	Findings             []Finding             `json:"findings,omitempty"`
	Triage               Triage                `json:"triage"`
	RawCommands          []CommandSnapshot     `json:"raw_commands,omitempty"`
	CollectedAt          time.Time             `json:"collected_at"`
}

type XAttr struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

type Finding struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
	Source     string   `json:"source"`
}

type Triage struct {
	Score              int            `json:"score"`
	Level              string         `json:"level"`
	Summary            string         `json:"summary"`
	Signals            []TriageSignal `json:"signals,omitempty"`
	RecommendedActions []string       `json:"recommended_actions,omitempty"`
}

type TriageSignal struct {
	Category string `json:"category"`
	Points   int    `json:"points"`
	Evidence string `json:"evidence"`
}

type CommandSnapshot struct {
	Command    []string `json:"command"`
	Stdout     string   `json:"stdout,omitempty"`
	Stderr     string   `json:"stderr,omitempty"`
	ExitCode   int      `json:"exit_code"`
	DurationMS int64    `json:"duration_ms"`
	Error      string   `json:"error,omitempty"`
}

func Analyze(ctx context.Context, inputPath string, opts Options) (Report, error) {
	if inputPath == "" {
		return Report{}, errors.New("usage: macscope macho [--json] [--full] <path>")
	}

	runner := opts.Runner
	if runner == nil {
		runner = collect.Runner{Timeout: 20 * time.Second}
	}

	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return Report{}, fmt.Errorf("resolve path: %w", err)
	}

	binaryPath, err := ResolveExecutable(ctx, absInput, runner)
	if err != nil {
		return Report{}, err
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		return Report{}, fmt.Errorf("stat binary: %w", err)
	}
	if info.IsDir() {
		return Report{}, fmt.Errorf("resolved binary is a directory: %s", binaryPath)
	}

	report := Report{
		InputPath:   absInput,
		BinaryPath:  binaryPath,
		SizeBytes:   info.Size(),
		CollectedAt: time.Now().UTC(),
	}

	report.SHA256, err = fileSHA256(binaryPath)
	if err != nil {
		return Report{}, fmt.Errorf("sha256: %w", err)
	}

	report.Architectures = ArchitecturesFromFile(binaryPath)

	fileResult, fileErr := runTool(ctx, runner, opts.Full, &report, "file", binaryPath)
	report.FileType = firstMeaningfulLine(fileResult.Stdout)
	if report.FileType == "" && fileErr != nil {
		report.FileType = fileErr.Error()
	}

	lipoResult, _ := runTool(ctx, runner, opts.Full, &report, "lipo", "-info", binaryPath)
	if archs := ParseLipoArchitectures(lipoResult.Stdout + "\n" + lipoResult.Stderr); len(archs) > 0 {
		report.Architectures = archs
	}

	codesignDetails, _ := runTool(ctx, runner, opts.Full, &report, "codesign", "-dvvv", "--entitlements", ":-", binaryPath)
	report.CodeSignature = codesign.ParseDetails(codesignDetails.Stdout, codesignDetails.Stderr)

	codesignVerify, _ := runTool(ctx, runner, opts.Full, &report, "codesign", "--verify", "--deep", "--strict", "--verbose=4", binaryPath)
	report.CodeSignatureVerify = codesign.ParseVerification(codesignVerify.Stdout, codesignVerify.Stderr, codesignVerify.ExitCode)

	spctlResult, _ := runTool(ctx, runner, opts.Full, &report, "spctl", "--assess", "--type", "execute", "--verbose=4", binaryPath)
	report.GatekeeperAssessment = gatekeeper.ParseAssessment(spctlResult.Stdout, spctlResult.Stderr, spctlResult.ExitCode)

	xattrResult, _ := runTool(ctx, runner, opts.Full, &report, "xattr", "-l", binaryPath)
	report.ExtendedAttributes = ParseXAttrs(xattrResult.Stdout + "\n" + xattrResult.Stderr)

	otoolResult, _ := runTool(ctx, runner, opts.Full, &report, "otool", "-L", binaryPath)
	report.LinkedLibraries = ParseLinkedLibraries(otoolResult.Stdout + "\n" + otoolResult.Stderr)

	report.Findings = classify(report)
	report.Triage = BuildTriage(report)

	return report, nil
}

func ResolveExecutable(ctx context.Context, path string, runner CommandRunner) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat target: %w", err)
	}
	if !info.IsDir() || !strings.HasSuffix(path, ".app") {
		return path, nil
	}

	infoPlist := filepath.Join(path, "Contents", "Info.plist")
	result, err := runner.Run(ctx, "/usr/libexec/PlistBuddy", "-c", "Print :CFBundleExecutable", infoPlist)
	if err == nil {
		exeName := strings.TrimSpace(result.Stdout)
		if exeName != "" {
			exePath := filepath.Join(path, "Contents", "MacOS", exeName)
			if _, statErr := os.Stat(exePath); statErr == nil {
				return exePath, nil
			}
		}
	}

	macOSDir := filepath.Join(path, "Contents", "MacOS")
	entries, readErr := os.ReadDir(macOSDir)
	if readErr != nil {
		return "", fmt.Errorf("resolve app executable: %w", readErr)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		candidate := filepath.Join(macOSDir, entry.Name())
		candidateInfo, statErr := os.Stat(candidate)
		if statErr == nil && candidateInfo.Mode()&0111 != 0 {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("resolve app executable: no executable found under %s", macOSDir)
}

func ParseLipoArchitectures(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}

	if idx := strings.Index(output, " are: "); idx >= 0 {
		return splitArchitectureList(output[idx+6:])
	}
	if idx := strings.Index(output, " architecture: "); idx >= 0 {
		return splitArchitectureList(output[idx+15:])
	}
	if idx := strings.Index(output, "Non-fat file:"); idx >= 0 {
		after := output[idx:]
		if archIdx := strings.Index(after, "is architecture:"); archIdx >= 0 {
			return splitArchitectureList(after[archIdx+16:])
		}
	}

	return nil
}

func ParseLinkedLibraries(output string) []string {
	var libs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		if idx := strings.Index(line, " ("); idx >= 0 {
			line = line[:idx]
		}
		if strings.HasPrefix(line, "/") || strings.HasPrefix(line, "@") {
			libs = append(libs, line)
		}
	}
	return uniqueSorted(libs)
}

func ParseXAttrs(output string) []XAttr {
	var attrs []XAttr
	var current *XAttr

	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			name, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			attrs = append(attrs, XAttr{
				Name:  strings.TrimSpace(name),
				Value: strings.TrimSpace(value),
			})
			current = &attrs[len(attrs)-1]
			continue
		}
		if current != nil {
			current.Value = strings.TrimSpace(current.Value + "\n" + strings.TrimSpace(line))
		}
	}

	return attrs
}

func ArchitecturesFromFile(path string) []string {
	fat, err := gomacho.OpenFat(path)
	if err == nil {
		defer fat.Close()
		var archs []string
		for _, arch := range fat.Arches {
			archs = append(archs, cpuName(arch.Cpu))
		}
		return uniqueSorted(archs)
	}

	file, err := gomacho.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	return []string{cpuName(file.Cpu)}
}

func runTool(ctx context.Context, runner CommandRunner, full bool, report *Report, name string, args ...string) (collect.Result, error) {
	result, err := runner.Run(ctx, name, args...)
	if full {
		snapshot := CommandSnapshot{
			Command:    result.Command,
			Stdout:     result.Stdout,
			Stderr:     result.Stderr,
			ExitCode:   result.ExitCode,
			DurationMS: result.Duration.Milliseconds(),
		}
		if err != nil {
			snapshot.Error = err.Error()
		}
		report.RawCommands = append(report.RawCommands, snapshot)
	}
	return result, err
}

func classify(report Report) []Finding {
	var findings []Finding

	verifyRaw := strings.ToLower(report.CodeSignatureVerify.Raw)
	if !report.CodeSignatureVerify.Valid && !isApplePlatformTrustQuirk(report, verifyRaw) {
		category := "INVALID_SIGNATURE"
		confidence := 0.82
		evidence := []string{"codesign verification returned a non-zero status"}
		if strings.Contains(verifyRaw, "not signed") || strings.Contains(verifyRaw, "code object is not signed") {
			category = "UNSIGNED_BINARY"
			confidence = 0.9
			evidence = append(evidence, "codesign reported the target is not signed")
		}
		if report.CodeSignatureVerify.Message != "" {
			evidence = append(evidence, report.CodeSignatureVerify.Message)
		}
		findings = append(findings, Finding{
			Category:   category,
			Severity:   "medium",
			Confidence: confidence,
			Evidence:   evidence,
			Source:     "codesign --verify --deep --strict --verbose=4",
		})
	}

	gatekeeperRaw := strings.ToLower(report.GatekeeperAssessment.Raw)
	gatekeeperPlatformQuirk := isApplePlatformBinary(report) && strings.Contains(gatekeeperRaw, "internal error in code signing subsystem")
	if !report.GatekeeperAssessment.Accepted && report.GatekeeperAssessment.Raw != "" && !gatekeeperPlatformQuirk {
		evidence := []string{"spctl assessment did not accept the target"}
		if report.GatekeeperAssessment.Source != "" {
			evidence = append(evidence, "source="+report.GatekeeperAssessment.Source)
		}
		findings = append(findings, Finding{
			Category:   "GATEKEEPER_REJECTED",
			Severity:   "medium",
			Confidence: 0.8,
			Evidence:   evidence,
			Source:     "spctl --assess --type execute --verbose=4",
		})
	}

	for _, attr := range report.ExtendedAttributes {
		if attr.Name == "com.apple.quarantine" {
			findings = append(findings, Finding{
				Category:   "QUARANTINE_PRESENT",
				Severity:   "low",
				Confidence: 0.95,
				Evidence:   []string{"com.apple.quarantine extended attribute is present"},
				Source:     "xattr -l",
			})
			break
		}
	}

	return findings
}

func BuildTriage(report Report) Triage {
	var signals []TriageSignal
	add := func(category string, points int, evidence string) {
		signals = append(signals, TriageSignal{
			Category: category,
			Points:   points,
			Evidence: evidence,
		})
	}

	for _, finding := range report.Findings {
		switch finding.Category {
		case "UNSIGNED_BINARY":
			add(finding.Category, 30, firstEvidence(finding.Evidence, "codesign reported unsigned binary"))
		case "INVALID_SIGNATURE":
			add(finding.Category, 25, firstEvidence(finding.Evidence, "codesign verification failed"))
		case "GATEKEEPER_REJECTED":
			add(finding.Category, 25, firstEvidence(finding.Evidence, "Gatekeeper assessment rejected target"))
		case "QUARANTINE_PRESENT":
			add(finding.Category, 8, firstEvidence(finding.Evidence, "quarantine xattr is present"))
		default:
			add(finding.Category, 10, firstEvidence(finding.Evidence, finding.Source))
		}
	}

	if riskPath := pathLocationSignal(report.BinaryPath); riskPath != "" {
		add("USER_WRITABLE_LOCATION", 10, riskPath)
	}
	for _, lib := range report.LinkedLibraries {
		if riskPath := pathLocationSignal(lib); riskPath != "" {
			add("USER_WRITABLE_LIBRARY_REFERENCE", 18, riskPath)
		}
	}
	if report.CodeSignature.Identifier == "" && report.CodeSignature.Raw != "" {
		add("MISSING_SIGNING_IDENTIFIER", 8, "codesign details did not include an identifier")
	}
	if len(report.Architectures) == 0 {
		add("UNKNOWN_ARCHITECTURE", 4, "architecture could not be determined")
	}

	score := 0
	for _, signal := range signals {
		score += signal.Points
	}
	if score > 100 {
		score = 100
	}

	level := triageLevel(score)
	return Triage{
		Score:              score,
		Level:              level,
		Summary:            triageSummary(level, score, len(signals)),
		Signals:            signals,
		RecommendedActions: triageActions(level, signals),
	}
}

func firstEvidence(values []string, fallback string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return fallback
}

func pathLocationSignal(path string) string {
	clean := filepath.Clean(path)
	switch {
	case strings.HasPrefix(clean, "/tmp/"):
		return "target path is under /tmp: " + clean
	case strings.HasPrefix(clean, "/private/tmp/"):
		return "target path is under /private/tmp: " + clean
	case strings.HasPrefix(clean, "/var/folders/"):
		return "target path is under /var/folders: " + clean
	case strings.HasPrefix(clean, "/Users/"):
		return "target path is under a user home directory: " + clean
	default:
		return ""
	}
}

func triageLevel(score int) string {
	switch {
	case score >= 75:
		return "CRITICAL"
	case score >= 50:
		return "HIGH"
	case score >= 20:
		return "MODERATE"
	default:
		return "LOW"
	}
}

func triageSummary(level string, score int, signalCount int) string {
	if signalCount == 0 {
		return "No notable signing, Gatekeeper, location, or quarantine signals were found."
	}
	return fmt.Sprintf("%s triage score %d from %d evidence-backed signal(s).", strings.ToLower(level), score, signalCount)
}

func triageActions(level string, signals []TriageSignal) []string {
	var actions []string
	if level == "LOW" {
		return []string{"No immediate action from static triage alone; preserve context if this file is part of an investigation."}
	}
	for _, signal := range signals {
		switch signal.Category {
		case "UNSIGNED_BINARY", "INVALID_SIGNATURE", "GATEKEEPER_REJECTED":
			actions = append(actions, "review signing and Gatekeeper evidence before execution")
		case "QUARANTINE_PRESENT":
			actions = append(actions, "preserve quarantine metadata and source context")
		case "USER_WRITABLE_LOCATION", "USER_WRITABLE_LIBRARY_REFERENCE":
			actions = append(actions, "review writable-path provenance and related files")
		}
	}
	return uniqueSorted(actions)
}

func isApplePlatformTrustQuirk(report Report, verifyRaw string) bool {
	return isApplePlatformBinary(report) && strings.Contains(verifyRaw, "cssmerr_tp_not_trusted")
}

func isApplePlatformBinary(report Report) bool {
	if report.CodeSignature.PlatformIdentifier != "" {
		return true
	}
	if strings.HasPrefix(report.CodeSignature.Identifier, "com.apple.") {
		return true
	}
	for _, authority := range report.CodeSignature.Authorities {
		if authority == "(unavailable)" && report.CodeSignature.TeamIdentifier == "not set" {
			return true
		}
	}
	return false
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func splitArchitectureList(value string) []string {
	fields := strings.Fields(value)
	for i, field := range fields {
		fields[i] = strings.Trim(field, ",")
	}
	return uniqueSorted(fields)
}

func firstMeaningfulLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func cpuName(cpu gomacho.Cpu) string {
	switch cpu {
	case gomacho.Cpu386:
		return "i386"
	case gomacho.CpuAmd64:
		return "x86_64"
	case gomacho.CpuArm:
		return "arm"
	case gomacho.CpuArm64:
		return "arm64"
	default:
		return cpu.String()
	}
}
