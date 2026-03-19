package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// DependenciesJson represents the structure of dependencies.json from jar files
type DependenciesJson struct {
	Version      string                `json:"version"`
	BuildTime    string                `json:"buildTime"`
	BuildMachine string                `json:"buildMachine"`
	BuildUser    string                `json:"buildUser,omitempty"`
	Hostname     string                `json:"hostname,omitempty"`
	SshPublicKey string                `json:"sshPublicKey,omitempty"`
	GitBranch    string                `json:"gitBranch"`
	GitCommit    string                `json:"gitCommit"`
	GitState     string                `json:"gitState"`
	Dependencies []DependencyJsonEntry `json:"dependencies"`
}

type DependencyJsonEntry struct {
	ModuleId  ModuleId         `json:"moduleId"`
	Resolver  string           `json:"resolver,omitempty"`
	Artifacts []ArtifactEntry  `json:"artifacts"`
}

type ModuleId struct {
	Organization string `json:"organization"`
	Artifact     string `json:"artifact"`
	Version      string `json:"version"`
}

type ArtifactEntry struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Extension string `json:"extension"`
	Repo      string `json:"repo,omitempty"`
	Path      string `json:"path,omitempty"`
	URL       string `json:"url,omitempty"`
	SHA256    string `json:"sha256"`
	Source    string `json:"source"`
}

// tryGetDependenciesFromJar attempts to get dependencies from dependencies.json published in the repo
// Returns the dependencies and version, or an error if not found
func tryGetDependenciesFromJar(repoConfig *RepoConfig, homeDir, organization, artifact, version string) ([]Dependency, string, error) {
	trace("Trying to get dependencies.json from repository")

	// Construct the dependencies.json URL
	depsURL := ConstructDependenciesJsonURL(repoConfig.URL, organization, artifact, version)
	trace("dependencies.json URL: %s", depsURL)

	// Fetch dependencies.json directly
	trace("Fetching dependencies.json...")
	depsJson, err := FetchDependenciesJson(depsURL, repoConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch dependencies.json: %w", err)
	}
	trace("Found dependencies.json with %d dependencies", len(depsJson.Dependencies))

	// Convert to Dependency structs
	dependencies, err := convertDependenciesJsonToDependencies(depsJson, homeDir, repoConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to convert dependencies: %w", err)
	}

	trace("Converted %d dependencies from dependencies.json", len(dependencies))
	return dependencies, depsJson.Version, nil
}

// ConstructDependenciesJsonURL builds the repository URL for dependencies.json
// Pattern: {repoURL}/{org-path}/{artifact}/{version}/{artifact}-{version}-dependencies.json
// Example: https://locus2.accur8.net/repos/all/io/accur8/a8-nefario_2.13/0.0.2-20251220_1053_master/a8-nefario_2.13-0.0.2-20251220_1053_master-dependencies.json
// Exported for use by other packages (e.g., godev)
func ConstructDependenciesJsonURL(repoBaseURL, organization, artifact, version string) string {
	// Convert organization to path (e.g., "io.accur8" -> "io/accur8", "a8" -> "a8")
	orgPath := strings.ReplaceAll(organization, ".", "/")

	// Build the URL: <repo>/<org-path>/<artifact>/<version>/<artifact>-<version>-dependencies.json
	depsFilename := fmt.Sprintf("%s-%s-dependencies.json", artifact, version)
	return fmt.Sprintf("%s/%s/%s/%s/%s",
		strings.TrimRight(repoBaseURL, "/"),
		orgPath,
		artifact,
		version,
		depsFilename)
}

// FetchDependenciesJson fetches and parses dependencies.json from a URL
// Exported for use by other packages (e.g., godev)
func FetchDependenciesJson(url string, repoConfig *RepoConfig) (*DependenciesJson, error) {
	// Create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add basic auth if credentials are provided
	if repoConfig.User != "" && repoConfig.Password != "" {
		req.SetBasicAuth(repoConfig.User, repoConfig.Password)
	}

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read and parse JSON
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var depsJson DependenciesJson
	if err := json.Unmarshal(data, &depsJson); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &depsJson, nil
}

// constructJarURL builds the Maven repository URL for a jar file
func constructJarURL(repoBaseURL, organization, artifact, version string) string {
	// Convert organization to path (e.g., "io.accur8" -> "io/accur8", "a8" -> "a8")
	orgPath := strings.ReplaceAll(organization, ".", "/")

	// Build the URL: <repo>/<org-path>/<artifact>/<version>/<artifact>-<version>.jar
	jarFilename := fmt.Sprintf("%s-%s.jar", artifact, version)
	return fmt.Sprintf("%s/%s/%s/%s/%s",
		strings.TrimRight(repoBaseURL, "/"),
		orgPath,
		artifact,
		version,
		jarFilename)
}

// downloadJarFile downloads a jar file using HTTP
// Returns the path to the downloaded file
func downloadJarFile(url string, repoConfig *RepoConfig) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "herman-*.jar")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	// Create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add basic auth if credentials are provided
	if repoConfig.User != "" && repoConfig.Password != "" {
		req.SetBasicAuth(repoConfig.User, repoConfig.Password)
	}

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpPath)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Write the response to the temp file
	outFile, err := os.Create(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return tmpPath, nil
}

// readDependenciesJsonFile reads and parses a dependencies.json file from disk
func readDependenciesJsonFile(path string) (*DependenciesJson, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var depsJson DependenciesJson
	if err := json.Unmarshal(data, &depsJson); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &depsJson, nil
}

// resolveRepoURL resolves a repo name to a URL
// Uses repo.properties for lookups with fallback defaults
func resolveRepoURL(homeDir, repoName string) (string, error) {
	// Default mappings
	defaults := map[string]string{
		"public_maven": "https://repo1.maven.org/maven2",
		"locus":        "https://locus.accur8.net/repos/all",
		"repo":         "https://locus.accur8.net/repos/all",
	}

	// Check if we have a default
	if defaultURL, ok := defaults[repoName]; ok {
		trace("Using default URL for repo '%s': %s", repoName, defaultURL)

		// Still check repo.properties in case user has overridden the default
		repoConfig, err := readRepoConfig(homeDir, repoName)
		if err == nil {
			trace("Found override in repo.properties for '%s': %s", repoName, repoConfig.URL)
			return repoConfig.URL, nil
		}

		return defaultURL, nil
	}

	// Not a default, must exist in repo.properties
	repoConfig, err := readRepoConfig(homeDir, repoName)
	if err != nil {
		return "", fmt.Errorf("repo '%s' not found in repo.properties and has no default: %w", repoName, err)
	}

	trace("Resolved repo '%s' to URL: %s", repoName, repoConfig.URL)
	return repoConfig.URL, nil
}

// constructArtifactURL builds the URL for an artifact
func constructArtifactURL(homeDir string, artifact *ArtifactEntry, moduleId *ModuleId) (string, error) {
	// If URL is already provided, use it
	if artifact.URL != "" {
		return artifact.URL, nil
	}

	// Determine repo name (default to "repo" if not specified)
	repoName := artifact.Repo
	if repoName == "" {
		repoName = "repo"
	}

	// Resolve repo to URL
	repoURL, err := resolveRepoURL(homeDir, repoName)
	if err != nil {
		return "", err
	}
	repoURL = strings.TrimRight(repoURL, "/")

	// If path is provided, use it
	if artifact.Path != "" {
		return fmt.Sprintf("%s/%s", repoURL, artifact.Path), nil
	}

	// Construct path from moduleId
	orgPath := strings.ReplaceAll(moduleId.Organization, ".", "/")
	artifactPath := fmt.Sprintf("%s/%s/%s/%s-%s.%s",
		orgPath,
		artifact.Name,
		moduleId.Version,
		artifact.Name,
		moduleId.Version,
		artifact.Extension)

	return fmt.Sprintf("%s/%s", repoURL, artifactPath), nil
}

// constructURLFromModule builds a URL from ModuleId when no artifact information is available
// This handles the case where we have module metadata but need to construct the artifact URL
// For io.accur8 modules, defaults to "repo". For others, requires explicit repo name.
func constructURLFromModule(homeDir string, moduleId *ModuleId, repoName string, classifier string) (string, error) {
	// If no repo name specified, default to "repo" for io.accur8 modules, otherwise error
	if repoName == "" {
		if moduleId.Organization == "io.accur8" {
			repoName = "repo"
		} else {
			return "", fmt.Errorf("cannot construct URL for %s:%s - no artifact information and no resolver specified",
				moduleId.Organization, moduleId.Artifact)
		}
	}

	// Resolve repo to URL
	repoURL, err := resolveRepoURL(homeDir, repoName)
	if err != nil {
		return "", fmt.Errorf("cannot construct URL for %s:%s: %w",
			moduleId.Organization, moduleId.Artifact, err)
	}
	repoURL = strings.TrimRight(repoURL, "/")

	// Build URL: {repoURL}/{org-path}/{artifact}/{version}/{artifact}-{version}[-classifier].jar
	orgPath := strings.ReplaceAll(moduleId.Organization, ".", "/")

	// Add classifier if provided
	classifierSuffix := ""
	if classifier != "" {
		classifierSuffix = "-" + classifier
	}

	filename := fmt.Sprintf("%s-%s%s.jar", moduleId.Artifact, moduleId.Version, classifierSuffix)
	url := fmt.Sprintf("%s/%s/%s/%s/%s",
		repoURL,
		orgPath,
		moduleId.Artifact,
		moduleId.Version,
		filename)

	return url, nil
}

// validateURLAndFetchHash validates that a URL is accessible and fetches its SHA256 hash
// First tries to use nix store prefetch-file (for public URLs)
// Falls back to downloading with auth if needed
func validateURLAndFetchHash(url string, repoConfig *RepoConfig) (string, error) {
	trace("Validating URL and fetching hash for: %s", url)

	// Try nix store prefetch-file first (works for public URLs, computes hash, adds to store)
	hash, err := fetchHashWithNixPrefetch(url)
	if err == nil {
		trace("Successfully fetched hash using nix store prefetch-file: %s", hash)
		return hash, nil
	}

	trace("nix store prefetch-file failed (%v), trying with auth...", err)

	// Fallback: validate URL with auth and download to compute hash
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add basic auth if credentials are provided
	if repoConfig.User != "" && repoConfig.Password != "" {
		req.SetBasicAuth(repoConfig.User, repoConfig.Password)
	}

	// Send the HEAD request to validate
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to validate URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("URL not accessible: HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	trace("URL validated with auth, downloading to compute SHA256...")

	// Download the file with auth
	tmpPath, err := downloadJarFile(url, repoConfig)
	if err != nil {
		return "", fmt.Errorf("failed to download for hash computation: %w", err)
	}
	defer os.Remove(tmpPath)

	// Re-use nix store prefetch-file on the local file to compute hash in correct format
	// This ensures we get the SRI format hash that matches what Nix expects
	hash, err = fetchHashWithNixPrefetch("file://" + tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to compute SHA256: %w", err)
	}

	trace("Computed SHA256: %s", hash)
	return hash, nil
}

// convertDependenciesJsonToDependencies converts DependenciesJson to []Dependency
func convertDependenciesJsonToDependencies(depsJson *DependenciesJson, homeDir string, repoConfig *RepoConfig) ([]Dependency, error) {
	var dependencies []Dependency

	for _, depEntry := range depsJson.Dependencies {
		// Find the jar artifact (type: jar OR extension: jar)
		// Some dependencies might only have one of these set
		var jarArtifact *ArtifactEntry
		for i, artifact := range depEntry.Artifacts {
			// Accept if type is jar OR extension is jar (or both)
			if artifact.Type == "jar" || artifact.Extension == "jar" {
				jarArtifact = &depEntry.Artifacts[i]
				break
			}
		}

		var url string
		var sha256 string
		var err error

		if jarArtifact == nil {
			// No artifact entry - try to construct URL from module metadata
			trace("No jar artifact found for %s:%s (has %d artifacts) - attempting to construct URL from module metadata",
				depEntry.ModuleId.Organization, depEntry.ModuleId.Artifact, len(depEntry.Artifacts))
			// Log what artifacts we did find
			for i, art := range depEntry.Artifacts {
				trace("  Artifact %d: type=%s, extension=%s, name=%s, url=%s",
					i, art.Type, art.Extension, art.Name, art.URL)
			}

			// Use resolver as repo name if available
			repoName := depEntry.Resolver
			url, err = constructURLFromModule(homeDir, &depEntry.ModuleId, repoName, "")
			if err != nil {
				return nil, fmt.Errorf("failed to construct URL for %s:%s: %w",
					depEntry.ModuleId.Organization, depEntry.ModuleId.Artifact, err)
			}

			// Validate URL and fetch SHA256
			trace("Validating constructed URL and fetching SHA256 for %s:%s",
				depEntry.ModuleId.Organization, depEntry.ModuleId.Artifact)
			sha256, err = validateURLAndFetchHash(url, repoConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to validate/fetch hash for %s:%s at %s: %w",
					depEntry.ModuleId.Organization, depEntry.ModuleId.Artifact, url, err)
			}
		} else {
			// Construct URL from artifact entry
			url, err = constructArtifactURL(homeDir, jarArtifact, &depEntry.ModuleId)
			if err != nil {
				return nil, fmt.Errorf("failed to construct URL for %s:%s: %w",
					depEntry.ModuleId.Organization, depEntry.ModuleId.Artifact, err)
			}

			// Convert SHA256 from hex to SRI format if needed
			sha256 = jarArtifact.SHA256
			if sha256 != "" && !strings.HasPrefix(sha256, "sha256-") {
				// Assume it's hex and convert to SRI
				sriHash, err := hexToSRI(sha256)
				if err != nil {
					return nil, fmt.Errorf("failed to convert hash for %s: %w",
						depEntry.ModuleId.Artifact, err)
				}
				sha256 = sriHash
			}
		}

		// Construct m2RepoPath
		orgPath := strings.ReplaceAll(depEntry.ModuleId.Organization, ".", "/")
		m2RepoPath := fmt.Sprintf("%s/%s/%s",
			orgPath,
			depEntry.ModuleId.Artifact,
			depEntry.ModuleId.Version)

		// Extract filename
		var filename string
		if jarArtifact != nil {
			filename = jarArtifact.Name + "-" + depEntry.ModuleId.Version + ".jar"
		} else {
			filename = depEntry.ModuleId.Artifact + "-" + depEntry.ModuleId.Version + ".jar"
		}

		// Create Dependency
		dep := Dependency{
			URL:          url,
			SHA256:       sha256,
			Organization: depEntry.ModuleId.Organization,
			Module:       depEntry.ModuleId.Artifact,
			Version:      depEntry.ModuleId.Version,
			M2RepoPath:   m2RepoPath,
			Filename:     filename,
		}

		dependencies = append(dependencies, dep)
	}

	return dependencies, nil
}

// prefetchJarWithNix downloads a jar using nix store prefetch-file to ensure it's in the Nix store
// and returns the hash
func prefetchJarWithNix(url string) (storePath string, hash string, err error) {
	trace("Prefetching jar with nix store prefetch-file: %s", url)

	// Run nix store prefetch-file
	cmd := exec.Command("nix", "store", "prefetch-file", url, "--json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", "", fmt.Errorf("nix store prefetch-file failed: %s", string(exitErr.Stderr))
		}
		return "", "", fmt.Errorf("failed to run nix store prefetch-file: %w", err)
	}

	// Parse JSON output
	var result struct {
		Hash      string `json:"hash"`
		StorePath string `json:"storePath"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse nix output: %w", err)
	}

	return result.StorePath, result.Hash, nil
}

// getJarPathFromNixStore gets the local file path of a jar in the Nix store
// by downloading it if necessary
func getJarPathFromNixStore(url string, repoConfig *RepoConfig) (string, error) {
	trace("Getting jar from Nix store: %s", url)

	// For now, we'll download to a temp file directly
	// In the future, we could use nix store prefetch-file, but that requires
	// the URL to be publicly accessible without auth
	return downloadJarFile(url, repoConfig)
}
