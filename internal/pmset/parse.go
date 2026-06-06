// Package pmset parses `pmset -g log` output into structured power-management
// events for sleep/wake/shutdown correlation. It is read-only: callers run
// pmset themselves and pass the captured output here for parsing.
package pmset

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// timeLayout matches the timestamp prefix pmset emits, e.g.
// "2026-05-30 14:08:54 -0400".
const timeLayout = "2006-01-02 15:04:05 -0700"

var (
	// lineRe captures the timestamp prefix and the remainder of a pmset log
	// line. The remainder holds the domain column and the message.
	lineRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} [+-]\d{4})\s+(.*)$`)
	// durationRe captures a trailing " <n> secs" duration that pmset appends
	// to sleep/wake records.
	durationRe = regexp.MustCompile(`(\d+)\s+secs\s*$`)
)

// Event is one parsed line from `pmset -g log`.
type Event struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	DurationSec int       `json:"duration_sec,omitempty"`
	Raw         string    `json:"raw"`
}

// String renders a compact one-line summary of the event.
func (e Event) String() string {
	stamp := e.Timestamp.Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("%s  %s", stamp, e.Type)
	if e.Message != "" {
		line += ": " + e.Message
	}
	if e.DurationSec > 0 {
		line += fmt.Sprintf(" (%ds)", e.DurationSec)
	}
	return line
}

// ParseLog parses the stdout of `pmset -g log` into events. Lines that do not
// begin with a recognizable timestamp are skipped.
func ParseLog(input string) []Event {
	var events []Event
	for _, raw := range strings.Split(input, "\n") {
		event, ok := parseLine(raw)
		if !ok {
			continue
		}
		events = append(events, event)
	}
	return events
}

func parseLine(raw string) (Event, bool) {
	trimmed := strings.TrimRight(raw, " \t")
	m := lineRe.FindStringSubmatch(trimmed)
	if m == nil {
		return Event{}, false
	}

	stamp, err := time.Parse(timeLayout, m[1])
	if err != nil {
		return Event{}, false
	}

	domain, message := splitDomain(m[2])

	event := Event{
		Timestamp: stamp,
		Type:      domain,
		Message:   message,
		Raw:       strings.TrimSpace(raw),
	}

	if dm := durationRe.FindStringSubmatch(message); dm != nil {
		// best-effort; the regex guarantees digits
		fmt.Sscanf(dm[1], "%d", &event.DurationSec)
		event.Message = strings.TrimSpace(durationRe.ReplaceAllString(message, ""))
	}

	return event, true
}

// splitDomain separates the padded domain column from the message. pmset
// terminates the domain column with a tab; when no tab is present the leading
// run of two-or-more spaces is used as the separator.
func splitDomain(rest string) (domain, message string) {
	if idx := strings.IndexByte(rest, '\t'); idx >= 0 {
		return strings.TrimSpace(rest[:idx]), strings.TrimSpace(rest[idx+1:])
	}
	if loc := regexp.MustCompile(`\s{2,}`).FindStringIndex(rest); loc != nil {
		return strings.TrimSpace(rest[:loc[0]]), strings.TrimSpace(rest[loc[1]:])
	}
	return strings.TrimSpace(rest), ""
}

// sleepWakeTypes is the set of domain columns considered sleep/wake/shutdown
// power transitions for correlation.
var sleepWakeTypes = map[string]bool{
	"sleep":     true,
	"wake":      true,
	"darkwake":  true,
	"shutdown":  true,
	"powerdown": true,
	"start":     true,
}

// SleepWakeEvents returns only the events whose type is a sleep, wake,
// darkwake, or shutdown power transition.
func SleepWakeEvents(events []Event) []Event {
	var filtered []Event
	for _, event := range events {
		if sleepWakeTypes[strings.ToLower(event.Type)] {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// Tail returns at most the last limit events.
func Tail(events []Event, limit int) []Event {
	if limit <= 0 || len(events) <= limit {
		return events
	}
	return events[len(events)-limit:]
}
