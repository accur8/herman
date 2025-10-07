package main

import (
	"encoding/xml"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name        string
		versionStr  string
		wantVersion string
		wantBuild   string
		wantBranch  string
		wantErr     bool
	}{
		{
			name:        "valid master branch version",
			versionStr:  "0.1.0-20250503_1316_master",
			wantVersion: "0.1.0",
			wantBuild:   "20250503_1316",
			wantBranch:  "master",
			wantErr:     false,
		},
		{
			name:        "valid feature branch version",
			versionStr:  "1.2.3-20220205_2049_feature-xyz",
			wantVersion: "1.2.3",
			wantBuild:   "20220205_2049",
			wantBranch:  "feature-xyz",
			wantErr:     false,
		},
		{
			name:        "valid dev branch version",
			versionStr:  "2.0.0-20230115_0930_dev",
			wantVersion: "2.0.0",
			wantBuild:   "20230115_0930",
			wantBranch:  "dev",
			wantErr:     false,
		},
		{
			name:       "invalid format - no build number",
			versionStr: "0.1.0-master",
			wantErr:    true,
		},
		{
			name:       "invalid format - wrong build number format",
			versionStr: "0.1.0-20250503-1316_master",
			wantErr:    true,
		},
		{
			name:       "invalid format - missing branch",
			versionStr: "0.1.0-20250503_1316",
			wantErr:    true,
		},
		{
			name:       "empty string",
			versionStr: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVersion(tt.versionStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Version != tt.wantVersion {
					t.Errorf("ParseVersion().Version = %v, want %v", got.Version, tt.wantVersion)
				}
				if got.BuildNumber != tt.wantBuild {
					t.Errorf("ParseVersion().BuildNumber = %v, want %v", got.BuildNumber, tt.wantBuild)
				}
				if got.Branch != tt.wantBranch {
					t.Errorf("ParseVersion().Branch = %v, want %v", got.Branch, tt.wantBranch)
				}
				if got.Full != tt.versionStr {
					t.Errorf("ParseVersion().Full = %v, want %v", got.Full, tt.versionStr)
				}
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want int // positive if v1 > v2, negative if v1 < v2, zero if equal
	}{
		{
			name: "equal versions",
			v1:   "1.0.0",
			v2:   "1.0.0",
			want: 0,
		},
		{
			name: "v1 greater - major version",
			v1:   "2.0.0",
			v2:   "1.0.0",
			want: 1, // positive
		},
		{
			name: "v2 greater - major version",
			v1:   "1.0.0",
			v2:   "2.0.0",
			want: -1, // negative
		},
		{
			name: "v1 greater - minor version",
			v1:   "1.2.0",
			v2:   "1.1.0",
			want: 1,
		},
		{
			name: "v1 greater - patch version",
			v1:   "1.0.5",
			v2:   "1.0.3",
			want: 1,
		},
		{
			name: "different length - v1 longer",
			v1:   "1.0.0.1",
			v2:   "1.0.0",
			want: 1,
		},
		{
			name: "different length - v2 longer",
			v1:   "1.0.0",
			v2:   "1.0.0.1",
			want: -1,
		},
		{
			name: "complex comparison",
			v1:   "10.2.3",
			v2:   "9.8.7",
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			// Check sign rather than exact value
			if (got > 0 && tt.want <= 0) || (got < 0 && tt.want >= 0) || (got == 0 && tt.want != 0) {
				t.Errorf("compareVersions(%v, %v) = %v, want sign of %v", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

func TestParseMavenMetadata(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<metadata>
  <groupId>io.accur8</groupId>
  <artifactId>a8-codegen_2.13</artifactId>
  <versioning>
    <latest>0.1.0-20250503_1316_master</latest>
    <versions>
      <version>0.1.0-20220205_2049_master</version>
      <version>0.1.0-20220331_1713_master</version>
      <version>0.1.0-20250503_1316_master</version>
      <version>1.0.0-20230115_0930_dev</version>
    </versions>
    <lastUpdated>20250503131600</lastUpdated>
  </versioning>
</metadata>`

	var metadata MavenMetadata
	err := xml.Unmarshal([]byte(xmlData), &metadata)
	if err != nil {
		t.Fatalf("Failed to parse maven metadata XML: %v", err)
	}

	if metadata.Versioning.Latest != "0.1.0-20250503_1316_master" {
		t.Errorf("Latest version = %v, want %v", metadata.Versioning.Latest, "0.1.0-20250503_1316_master")
	}

	if len(metadata.Versioning.Versions) != 4 {
		t.Errorf("Number of versions = %v, want %v", len(metadata.Versioning.Versions), 4)
	}

	expectedVersions := []string{
		"0.1.0-20220205_2049_master",
		"0.1.0-20220331_1713_master",
		"0.1.0-20250503_1316_master",
		"1.0.0-20230115_0930_dev",
	}

	for i, expected := range expectedVersions {
		if metadata.Versioning.Versions[i] != expected {
			t.Errorf("Version[%d] = %v, want %v", i, metadata.Versioning.Versions[i], expected)
		}
	}
}

func TestFindLatestVersion(t *testing.T) {
	metadata := &MavenMetadata{
		Versioning: struct {
			Latest   string   `xml:"latest"`
			Versions []string `xml:"versions>version"`
		}{
			Latest: "0.1.0-20250503_1316_master",
			Versions: []string{
				"0.1.0-20220205_2049_master",
				"0.1.0-20220331_1713_master",
				"0.2.0-20230115_0930_master",
				"0.2.0-20230115_1100_master", // Same version, later build
				"0.1.0-20250503_1316_master",
				"1.0.0-20230115_0930_dev",
				"1.0.0-20230116_0930_dev", // Later build on dev
			},
		},
	}

	tests := []struct {
		name    string
		branch  string
		want    string
		wantErr bool
	}{
		{
			name:    "latest master version",
			branch:  "master",
			want:    "0.2.0-20230115_1100_master", // Highest version (0.2.0), latest build
			wantErr: false,
		},
		{
			name:    "latest dev version",
			branch:  "dev",
			want:    "1.0.0-20230116_0930_dev", // Only version on dev, latest build
			wantErr: false,
		},
		{
			name:    "non-existent branch",
			branch:  "feature-xyz",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindLatestVersion(metadata, tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindLatestVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("FindLatestVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindLatestVersionSorting(t *testing.T) {
	// Test that versions are properly sorted by version number first, then build number
	metadata := &MavenMetadata{
		Versioning: struct {
			Latest   string   `xml:"latest"`
			Versions []string `xml:"versions>version"`
		}{
			Versions: []string{
				"0.1.0-20220101_0000_master",
				"0.1.1-20220101_0000_master",
				"0.1.1-20220102_0000_master", // Same version, later build - should win for 0.1.x
				"0.2.0-20220101_0000_master",
				"0.10.0-20220101_0000_master", // Should be > 0.2.0
				"1.0.0-20220101_0000_master",  // Should be the latest
			},
		},
	}

	got, err := FindLatestVersion(metadata, "master")
	if err != nil {
		t.Fatalf("FindLatestVersion() error = %v", err)
	}

	want := "1.0.0-20220101_0000_master"
	if got != want {
		t.Errorf("FindLatestVersion() = %v, want %v", got, want)
	}
}
