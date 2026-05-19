package codesign

import (
	"strings"
)

type Details struct {
	Identifier             string   `json:"identifier,omitempty"`
	Format                 string   `json:"format,omitempty"`
	CodeDirectoryFlags     string   `json:"code_directory_flags,omitempty"`
	PlatformIdentifier     string   `json:"platform_identifier,omitempty"`
	TeamIdentifier         string   `json:"team_identifier,omitempty"`
	RuntimeVersion         string   `json:"runtime_version,omitempty"`
	SealedResourcesVersion string   `json:"sealed_resources_version,omitempty"`
	Authorities            []string `json:"authorities,omitempty"`
	Entitlements           string   `json:"entitlements,omitempty"`
	Raw                    string   `json:"raw,omitempty"`
}

type Verification struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
	Raw     string `json:"raw,omitempty"`
}

func ParseDetails(stdout, stderr string) Details {
	raw := strings.TrimSpace(stdout + "\n" + stderr)
	details := Details{Raw: raw}
	details.Entitlements = plistFrom(stdout)

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "<?xml") || strings.HasPrefix(line, "<!DOCTYPE") || strings.HasPrefix(line, "<plist") {
			if details.Entitlements == "" {
				details.Entitlements = plistFrom(raw)
			}
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "Identifier":
			details.Identifier = value
		case "Format":
			details.Format = value
		case "CodeDirectory flags":
			details.CodeDirectoryFlags = value
		case "Platform identifier":
			details.PlatformIdentifier = value
		case "TeamIdentifier":
			details.TeamIdentifier = value
		case "Runtime Version":
			details.RuntimeVersion = value
		case "Sealed Resources version":
			details.SealedResourcesVersion = value
		case "Authority":
			details.Authorities = append(details.Authorities, value)
		}
	}

	return details
}

func ParseVerification(stdout, stderr string, exitCode int) Verification {
	raw := strings.TrimSpace(stdout + "\n" + stderr)
	return Verification{
		Valid:   exitCode == 0,
		Message: firstMeaningfulLine(raw),
		Raw:     raw,
	}
}

func plistFrom(raw string) string {
	start := strings.Index(raw, "<?xml")
	if start < 0 {
		start = strings.Index(raw, "<plist")
	}
	if start < 0 {
		return ""
	}
	return strings.TrimSpace(raw[start:])
}

func firstMeaningfulLine(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
