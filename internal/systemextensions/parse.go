package systemextensions

import (
	"regexp"
	"strings"
)

// ExtensionType maps to the com.apple.system_extension.* suffix.
type ExtensionType string

const (
	TypeNetwork     ExtensionType = "network_extension"
	TypeEndpointSec ExtensionType = "endpoint_security"
	TypeDriver      ExtensionType = "driver_extension"
	TypeUnknown     ExtensionType = "unknown"
)

// Extension represents one row from systemextensionsctl list.
type Extension struct {
	Type     ExtensionType `json:"type"`
	Enabled  bool          `json:"enabled"`
	Active   bool          `json:"active"`
	TeamID   string        `json:"team_id"`
	BundleID string        `json:"bundle_id"`
	Version  string        `json:"version,omitempty"`
	Name     string        `json:"name"`
	State    string        `json:"state"`
}

var (
	// macOS may append advisory text after the type, e.g.:
	// --- com.apple.system_extension.network_extension (Go to 'System Settings...') ---
	sectionRe       = regexp.MustCompile(`---\s+com\.apple\.system_extension\.(\w+)`)
	bundleVersionRe = regexp.MustCompile(`^(.+?)\s+\(([^)]+)\)$`)
)

// ParseList parses the stdout of `systemextensionsctl list`.
func ParseList(input string) []Extension {
	var extensions []Extension
	currentType := TypeUnknown

	for _, line := range strings.Split(input, "\n") {
		if m := sectionRe.FindStringSubmatch(line); m != nil {
			currentType = extensionType(m[1])
			continue
		}
		// Skip the column header row.
		if strings.HasPrefix(strings.TrimSpace(line), "enabled") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		enabled := strings.TrimSpace(fields[0]) == "*"
		active := strings.TrimSpace(fields[1]) == "*"
		teamID := strings.TrimSpace(fields[2])
		bundleStr := strings.TrimSpace(fields[3])
		name := strings.TrimSpace(fields[4])
		state := ""
		if len(fields) >= 6 {
			state = strings.Trim(strings.TrimSpace(fields[5]), "[]")
		}

		if teamID == "" && bundleStr == "" {
			continue
		}

		bundleID, version := splitBundleVersion(bundleStr)

		extensions = append(extensions, Extension{
			Type:     currentType,
			Enabled:  enabled,
			Active:   active,
			TeamID:   teamID,
			BundleID: bundleID,
			Version:  version,
			Name:     name,
			State:    state,
		})
	}
	return extensions
}

func extensionType(suffix string) ExtensionType {
	switch suffix {
	case "network_extension":
		return TypeNetwork
	case "endpoint_security":
		return TypeEndpointSec
	case "driver_extension":
		return TypeDriver
	default:
		return TypeUnknown
	}
}

func splitBundleVersion(s string) (bundleID, version string) {
	if m := bundleVersionRe.FindStringSubmatch(s); m != nil {
		return m[1], m[2]
	}
	return s, ""
}
