package gatekeeper

import "strings"

type Assessment struct {
	Accepted bool     `json:"accepted"`
	Source   string   `json:"source,omitempty"`
	Origin   string   `json:"origin,omitempty"`
	Messages []string `json:"messages,omitempty"`
	Raw      string   `json:"raw,omitempty"`
}

func ParseAssessment(stdout, stderr string, exitCode int) Assessment {
	raw := strings.TrimSpace(stdout + "\n" + stderr)
	assessment := Assessment{
		Accepted: exitCode == 0 && strings.Contains(strings.ToLower(raw), "accepted"),
		Raw:      raw,
	}

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if ok {
			switch strings.TrimSpace(key) {
			case "source":
				assessment.Source = strings.TrimSpace(value)
			case "origin":
				assessment.Origin = strings.TrimSpace(value)
			default:
				assessment.Messages = append(assessment.Messages, line)
			}
			continue
		}

		assessment.Messages = append(assessment.Messages, line)
	}

	return assessment
}
