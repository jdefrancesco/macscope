package macho

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jdefrancesco/macscope/internal/codesign"
	"github.com/jdefrancesco/macscope/internal/collect"
	"github.com/jdefrancesco/macscope/internal/gatekeeper"
)

type fakeRunner struct{}

func (fakeRunner) Run(_ context.Context, name string, args ...string) (collect.Result, error) {
	result := collect.Result{
		Command:  append([]string{name}, args...),
		ExitCode: 0,
		Duration: time.Millisecond,
	}

	switch name {
	case "file":
		result.Stdout = args[len(args)-1] + ": Mach-O 64-bit executable arm64\n"
	case "lipo":
		result.Stdout = "Non-fat file: target is architecture: arm64\n"
	case "codesign":
		if len(args) > 0 && args[0] == "-dvvv" {
			result.Stderr = "Identifier=com.example.tool\nFormat=Mach-O thin (arm64)\nTeamIdentifier=TEAM123\nAuthority=Developer ID Application\n"
		} else {
			result.Stderr = args[len(args)-1] + ": valid on disk\n"
		}
	case "spctl":
		result.Stdout = args[len(args)-1] + ": accepted\nsource=Developer ID\norigin=Developer ID Application\n"
	case "xattr":
		result.Stdout = "com.apple.quarantine: 0081;00000000;Safari;\n"
	case "otool":
		result.Stdout = args[len(args)-1] + ":\n\t/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1336.0.0)\n\t@rpath/libExample.dylib (compatibility version 1.0.0, current version 1.2.3)\n"
	default:
		result.ExitCode = 127
		result.Stderr = "not found"
	}

	return result, nil
}

func TestAnalyzeFile(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "tool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("not really a macho"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	report, err := Analyze(context.Background(), file.Name(), Options{
		Full:   true,
		Runner: fakeRunner{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if report.InputPath == "" || report.BinaryPath != report.InputPath {
		t.Fatalf("paths = input %q binary %q, want same absolute file path", report.InputPath, report.BinaryPath)
	}
	if report.SHA256 == "" {
		t.Fatal("SHA256 = empty, want hash")
	}
	if len(report.Architectures) != 1 || report.Architectures[0] != "arm64" {
		t.Fatalf("Architectures = %#v, want arm64", report.Architectures)
	}
	if report.CodeSignature.Identifier != "com.example.tool" {
		t.Fatalf("CodeSignature.Identifier = %q, want com.example.tool", report.CodeSignature.Identifier)
	}
	if !report.CodeSignatureVerify.Valid {
		t.Fatal("CodeSignatureVerify.Valid = false, want true")
	}
	if !report.GatekeeperAssessment.Accepted {
		t.Fatal("GatekeeperAssessment.Accepted = false, want true")
	}
	if len(report.LinkedLibraries) != 2 {
		t.Fatalf("LinkedLibraries = %#v, want 2 libs", report.LinkedLibraries)
	}
	if !hasFinding(report.Findings, "QUARANTINE_PRESENT") {
		t.Fatalf("Findings = %#v, want QUARANTINE_PRESENT", report.Findings)
	}
	if report.Triage.Score == 0 || report.Triage.Level == "" {
		t.Fatalf("Triage = %#v, want non-zero score and level", report.Triage)
	}
	if len(report.RawCommands) == 0 {
		t.Fatal("RawCommands = empty, want command snapshots when Full is true")
	}
}

func TestParseLipoArchitectures(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wanted []string
	}{
		{
			name:   "fat",
			input:  "Architectures in the fat file: tool are: x86_64 arm64",
			wanted: []string{"arm64", "x86_64"},
		},
		{
			name:   "non fat",
			input:  "Non-fat file: tool is architecture: arm64",
			wanted: []string{"arm64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLipoArchitectures(tt.input)
			if strings.Join(got, ",") != strings.Join(tt.wanted, ",") {
				t.Fatalf("ParseLipoArchitectures() = %#v, want %#v", got, tt.wanted)
			}
		})
	}
}

func TestParseLinkedLibraries(t *testing.T) {
	input := `tool:
	/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1336.0.0)
	@rpath/libThing.dylib (compatibility version 1.0.0, current version 1.0.0)`

	got := ParseLinkedLibraries(input)
	if strings.Join(got, ",") != "/usr/lib/libSystem.B.dylib,@rpath/libThing.dylib" {
		t.Fatalf("ParseLinkedLibraries() = %#v", got)
	}
}

func TestParseXAttrs(t *testing.T) {
	got := ParseXAttrs("com.apple.quarantine: 0081;Safari;\ncom.apple.metadata:kMDItemWhereFroms:\n\t00000000\n")
	if len(got) != 2 {
		t.Fatalf("ParseXAttrs length = %d, want 2", len(got))
	}
	if got[0].Name != "com.apple.quarantine" {
		t.Fatalf("first xattr = %q, want quarantine", got[0].Name)
	}
	if !strings.Contains(got[1].Value, "00000000") {
		t.Fatalf("second xattr value = %q, want continuation", got[1].Value)
	}
}

func TestClassifySkipsApplePlatformTrustQuirk(t *testing.T) {
	report := Report{
		CodeSignature: codesign.Details{
			Identifier:         "com.apple.ls",
			PlatformIdentifier: "26",
			TeamIdentifier:     "not set",
			Authorities:        []string{"(unavailable)"},
		},
		CodeSignatureVerify: codesign.Verification{
			Valid: false,
			Raw:   "/bin/ls: CSSMERR_TP_NOT_TRUSTED",
		},
		GatekeeperAssessment: gatekeeper.Assessment{
			Accepted: false,
			Raw:      "/bin/ls: internal error in Code Signing subsystem",
		},
	}

	if got := classify(report); len(got) != 0 {
		t.Fatalf("classify() = %#v, want no findings for Apple platform trust-policy quirk", got)
	}
}

func TestBuildTriageScoresFindings(t *testing.T) {
	report := Report{
		BinaryPath:      "/Users/alice/Downloads/tool",
		Architectures:   []string{"arm64"},
		LinkedLibraries: []string{"/tmp/libOdd.dylib"},
		Findings: []Finding{
			{
				Category: "UNSIGNED_BINARY",
				Evidence: []string{"codesign reported the target is not signed"},
			},
			{
				Category: "GATEKEEPER_REJECTED",
				Evidence: []string{"spctl assessment did not accept the target"},
			},
		},
	}

	triage := BuildTriage(report)
	if triage.Score < 75 {
		t.Fatalf("Score = %d, want high score", triage.Score)
	}
	if triage.Level != "CRITICAL" {
		t.Fatalf("Level = %q, want CRITICAL", triage.Level)
	}
	if len(triage.Signals) < 4 {
		t.Fatalf("Signals = %#v, want findings plus path signals", triage.Signals)
	}
}

func hasFinding(findings []Finding, category string) bool {
	for _, finding := range findings {
		if finding.Category == category {
			return true
		}
	}
	return false
}
