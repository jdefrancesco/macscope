package codesign

import (
	"strings"
	"testing"
)

func TestParseDetails(t *testing.T) {
	input := `Executable=/bin/ls
Identifier=com.apple.ls
Format=Mach-O thin (arm64e)
CodeDirectory flags=0x0(none)
Platform identifier=26
Runtime Version=14.0.0
Sealed Resources version=2 rules=13 files=0
Authority=Software Signing
Authority=Apple Code Signing Certification Authority
TeamIdentifier=not set
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
</plist>`

	got := ParseDetails("", input)

	if got.Identifier != "com.apple.ls" {
		t.Fatalf("Identifier = %q, want com.apple.ls", got.Identifier)
	}
	if got.Format != "Mach-O thin (arm64e)" {
		t.Fatalf("Format = %q, want Mach-O thin", got.Format)
	}
	if got.TeamIdentifier != "not set" {
		t.Fatalf("TeamIdentifier = %q, want not set", got.TeamIdentifier)
	}
	if got.PlatformIdentifier != "26" {
		t.Fatalf("PlatformIdentifier = %q, want 26", got.PlatformIdentifier)
	}
	if len(got.Authorities) != 2 {
		t.Fatalf("Authorities length = %d, want 2", len(got.Authorities))
	}
	if got.Entitlements == "" {
		t.Fatal("Entitlements = empty, want plist")
	}
}

func TestParseDetailsKeepsEntitlementsFromStdout(t *testing.T) {
	stdout := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
</plist>`
	stderr := "Identifier=com.example.tool\nTeamIdentifier=TEAM123\n"

	got := ParseDetails(stdout, stderr)

	if got.Identifier != "com.example.tool" {
		t.Fatalf("Identifier = %q, want com.example.tool", got.Identifier)
	}
	if got.TeamIdentifier != "TEAM123" {
		t.Fatalf("TeamIdentifier = %q, want TEAM123", got.TeamIdentifier)
	}
	if strings.Contains(got.Entitlements, "Identifier=") {
		t.Fatalf("Entitlements included stderr metadata: %q", got.Entitlements)
	}
}

func TestParseVerification(t *testing.T) {
	tests := []struct {
		name     string
		stderr   string
		exitCode int
		valid    bool
		message  string
	}{
		{
			name:     "valid",
			stderr:   "/bin/ls: valid on disk\n/bin/ls: satisfies its Designated Requirement",
			exitCode: 0,
			valid:    true,
			message:  "/bin/ls: valid on disk",
		},
		{
			name:     "unsigned",
			stderr:   "/tmp/tool: code object is not signed at all",
			exitCode: 1,
			valid:    false,
			message:  "/tmp/tool: code object is not signed at all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseVerification("", tt.stderr, tt.exitCode)
			if got.Valid != tt.valid {
				t.Fatalf("Valid = %v, want %v", got.Valid, tt.valid)
			}
			if got.Message != tt.message {
				t.Fatalf("Message = %q, want %q", got.Message, tt.message)
			}
		})
	}
}
