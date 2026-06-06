package systemextensions

import "testing"

const sampleOutput = `--- com.apple.system_extension.network_extension ---
enabled	active	teamID	bundleID (version)	name	[state]
*	*	UBF8T346G9	com.microsoft.wdav.netext (101.76.90)	Microsoft Defender Network Ext	[activated enabled]
*		XXXXXXXXXX	com.example.vpn.ext (2.1.0)	Example VPN	[activated waiting for user]

--- com.apple.system_extension.endpoint_security ---
enabled	active	teamID	bundleID (version)	name	[state]
*	*	UBF8T346G9	com.microsoft.wdav.epsext (101.76.90)	Microsoft Defender ES	[activated enabled]

--- com.apple.system_extension.driver_extension ---
enabled	active	teamID	bundleID (version)	name	[state]
*	*	TEAMID1234	com.example.driver (1.0)	Example Driver	[activated enabled]
`

func TestParseList(t *testing.T) {
	exts := ParseList(sampleOutput)
	if len(exts) != 4 {
		t.Fatalf("got %d extensions, want 4", len(exts))
	}

	tests := []struct {
		idx      int
		wantType ExtensionType
		wantID   string
		enabled  bool
		active   bool
		state    string
	}{
		{0, TypeNetwork, "com.microsoft.wdav.netext", true, true, "activated enabled"},
		{1, TypeNetwork, "com.example.vpn.ext", true, false, "activated waiting for user"},
		{2, TypeEndpointSec, "com.microsoft.wdav.epsext", true, true, "activated enabled"},
		{3, TypeDriver, "com.example.driver", true, true, "activated enabled"},
	}

	for _, tt := range tests {
		ext := exts[tt.idx]
		if ext.Type != tt.wantType {
			t.Errorf("[%d] type = %q, want %q", tt.idx, ext.Type, tt.wantType)
		}
		if ext.BundleID != tt.wantID {
			t.Errorf("[%d] bundleID = %q, want %q", tt.idx, ext.BundleID, tt.wantID)
		}
		if ext.Enabled != tt.enabled {
			t.Errorf("[%d] enabled = %v, want %v", tt.idx, ext.Enabled, tt.enabled)
		}
		if ext.Active != tt.active {
			t.Errorf("[%d] active = %v, want %v", tt.idx, ext.Active, tt.active)
		}
		if ext.State != tt.state {
			t.Errorf("[%d] state = %q, want %q", tt.idx, ext.State, tt.state)
		}
	}
}

func TestSplitBundleVersion(t *testing.T) {
	tests := []struct {
		input   string
		wantID  string
		wantVer string
	}{
		{"com.example.ext (1.0)", "com.example.ext", "1.0"},
		{"com.example.ext (101.76.90)", "com.example.ext", "101.76.90"},
		{"com.example.ext", "com.example.ext", ""},
	}
	for _, tt := range tests {
		id, ver := splitBundleVersion(tt.input)
		if id != tt.wantID || ver != tt.wantVer {
			t.Errorf("splitBundleVersion(%q) = (%q, %q), want (%q, %q)", tt.input, id, ver, tt.wantID, tt.wantVer)
		}
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		extensions []Extension
		wantCat    string
	}{
		{
			name: "multiple network extensions",
			extensions: []Extension{
				{Type: TypeNetwork, Enabled: true, BundleID: "com.a.vpn", Name: "VPN A", State: "activated enabled"},
				{Type: TypeNetwork, Enabled: true, BundleID: "com.b.vpn", Name: "VPN B", State: "activated enabled"},
			},
			wantCat: "MULTIPLE_NETWORK_EXTENSIONS",
		},
		{
			name: "extension awaiting approval",
			extensions: []Extension{
				{Type: TypeNetwork, Enabled: true, BundleID: "com.a.vpn", Name: "VPN", State: "activated waiting for user"},
			},
			wantCat: "EXTENSION_AWAITING_APPROVAL",
		},
		{
			name: "terminated extension",
			extensions: []Extension{
				{Type: TypeEndpointSec, Enabled: true, BundleID: "com.a.es", Name: "ES", State: "terminated waiting for removal"},
			},
			wantCat: "EXTENSION_TERMINATED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := Classify(tt.extensions)
			if !hasFinding(findings, tt.wantCat) {
				t.Errorf("findings = %v, want category %q", findingCategories(findings), tt.wantCat)
			}
		})
	}
}

func TestClassifyClean(t *testing.T) {
	exts := []Extension{
		{Type: TypeNetwork, Enabled: true, Active: true, BundleID: "com.a.vpn", Name: "VPN", State: "activated enabled"},
		{Type: TypeEndpointSec, Enabled: true, Active: true, BundleID: "com.a.es", Name: "ES", State: "activated enabled"},
	}
	findings := Classify(exts)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %v", findingCategories(findings))
	}
}

func hasFinding(findings []Finding, category string) bool {
	for _, f := range findings {
		if f.Category == category {
			return true
		}
	}
	return false
}

func findingCategories(findings []Finding) []string {
	cats := make([]string, len(findings))
	for i, f := range findings {
		cats[i] = f.Category
	}
	return cats
}
