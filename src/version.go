package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// VersionInfo represents a parsed version string: VERSION-BUILDNUMBER_BRANCH
type VersionInfo struct {
	Full        string // The complete version string
	Version     string // e.g., "0.1.0"
	BuildNumber string // e.g., "20250503_1316"
	Branch      string // e.g., "master"
}

// MavenMetadata represents the structure of maven-metadata.xml
type MavenMetadata struct {
	Versioning struct {
		Latest   string   `xml:"latest"`
		Versions []string `xml:"versions>version"`
	} `xml:"versioning"`
}

// ParseVersion parses a version string in the format VERSION-BUILDNUMBER_BRANCH
// Example: "0.1.0-20250503_1316_master" -> Version: "0.1.0", BuildNumber: "20250503_1316", Branch: "master"
func ParseVersion(versionStr string) (*VersionInfo, error) {
	// Pattern: VERSION-BUILDNUMBER_BRANCH
	// Where BUILDNUMBER is YYYYMMDD_HHMM
	pattern := regexp.MustCompile(`^([^-]+)-(\d{8}_\d{4})_(.+)$`)
	matches := pattern.FindStringSubmatch(versionStr)

	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid version format: %s (expected VERSION-BUILDNUMBER_BRANCH)", versionStr)
	}

	return &VersionInfo{
		Full:        versionStr,
		Version:     matches[1],
		BuildNumber: matches[2],
		Branch:      matches[3],
	}, nil
}

// FetchMavenMetadata fetches and parses maven-metadata.xml from the repository
func FetchMavenMetadata(repoConfig *RepoConfig, organization, artifact string) (*MavenMetadata, error) {
	// Convert organization to path (e.g., "io.accur8" -> "io/accur8")
	orgPath := strings.ReplaceAll(organization, ".", "/")

	// Build URL: https://locus.accur8.net/repos/all/io/accur8/a8-codegen_2.13/maven-metadata.xml
	baseURL := strings.TrimRight(repoConfig.URL, "/")
	metadataURL := fmt.Sprintf("%s/%s/%s/maven-metadata.xml", baseURL, orgPath, artifact)

	trace("Fetching maven-metadata.xml from: %s", metadataURL)

	req, err := http.NewRequest("GET", metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add basic auth if credentials are provided
	if repoConfig.User != "" && repoConfig.Password != "" {
		req.SetBasicAuth(repoConfig.User, repoConfig.Password)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch maven-metadata.xml: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch maven-metadata.xml (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read maven-metadata.xml: %w", err)
	}

	var metadata MavenMetadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse maven-metadata.xml: %w", err)
	}

	trace("Found %d versions in maven-metadata.xml", len(metadata.Versioning.Versions))
	return &metadata, nil
}

// FindLatestVersion finds the latest version for a given branch
// Algorithm:
// 1. Filter versions by branch
// 2. Find the highest version number
// 3. If multiple builds exist for that version, find the highest build number
func FindLatestVersion(metadata *MavenMetadata, branch string) (string, error) {
	var versionsForBranch []VersionInfo

	// Parse all versions and filter by branch
	for _, versionStr := range metadata.Versioning.Versions {
		versionInfo, err := ParseVersion(versionStr)
		if err != nil {
			trace("Skipping invalid version: %s (%v)", versionStr, err)
			continue
		}

		if versionInfo.Branch == branch {
			versionsForBranch = append(versionsForBranch, *versionInfo)
		}
	}

	if len(versionsForBranch) == 0 {
		return "", fmt.Errorf("no versions found for branch: %s", branch)
	}

	trace("Found %d versions for branch %s", len(versionsForBranch), branch)

	// Sort by version number (descending), then build number (descending)
	sort.Slice(versionsForBranch, func(i, j int) bool {
		// Compare versions
		versionCmp := compareVersions(versionsForBranch[i].Version, versionsForBranch[j].Version)
		if versionCmp != 0 {
			return versionCmp > 0 // Higher version first
		}

		// Same version, compare build numbers
		return versionsForBranch[i].BuildNumber > versionsForBranch[j].BuildNumber
	})

	latestVersion := versionsForBranch[0].Full
	trace("Latest version for branch %s: %s", branch, latestVersion)
	return latestVersion, nil
}

// compareVersions compares two version strings (e.g., "0.1.0" vs "0.2.0")
// Returns: positive if v1 > v2, negative if v1 < v2, zero if equal
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int

		if i < len(parts1) {
			p1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			p2, _ = strconv.Atoi(parts2[i])
		}

		if p1 != p2 {
			return p1 - p2
		}
	}

	return 0
}
