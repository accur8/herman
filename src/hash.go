package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// FetchMissingHashes fetches SHA256 hashes for dependencies that don't have them
// If useNixPrefetch is true, uses nix store prefetch-file (for installation)
// If useNixPrefetch is false, fetches from Maven repository .sha256 files (for Nix generation)
func FetchMissingHashes(dependencies []Dependency, useNixPrefetch bool) ([]Dependency, error) {
	// Check if we need to fetch any hashes
	needsFetch := false
	for _, dep := range dependencies {
		if dep.SHA256 == "" {
			needsFetch = true
			break
		}
	}

	if !needsFetch {
		trace("All dependencies have SHA256 hashes")
		return dependencies, nil
	}

	trace("Fetching missing SHA256 hashes for %d dependencies", len(dependencies))
	fmt.Fprintf(os.Stderr, "Fetching SHA256 hashes...\n")

	// Create a copy of dependencies to fill in
	result := make([]Dependency, len(dependencies))
	copy(result, dependencies)

	// Use a wait group and channels for parallel fetching
	var wg sync.WaitGroup
	type hashResult struct {
		index int
		hash  string
		err   error
	}
	resultChan := make(chan hashResult, len(dependencies))

	// Count how many we're fetching
	fetchCount := 0
	for i, dep := range result {
		if dep.SHA256 == "" {
			fetchCount++
			wg.Add(1)
			go func(idx int, url string) {
				defer wg.Done()

				var hash string
				var err error
				if useNixPrefetch {
					hash, err = fetchHashWithNixPrefetch(url)
				} else {
					hash, err = fetchHashFromMavenRepo(url)
				}

				if err != nil {
					resultChan <- hashResult{index: idx, err: fmt.Errorf("failed to fetch hash for %s: %w", url, err)}
					return
				}

				resultChan <- hashResult{index: idx, hash: hash}
			}(i, dep.URL)
		}
	}

	// Wait for all fetches to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and check for errors
	for res := range resultChan {
		if res.err != nil {
			return nil, res.err
		}
		result[res.index].SHA256 = res.hash
		trace("Fetched hash for %s: %s", result[res.index].URL, res.hash)
	}

	if fetchCount > 0 {
		fmt.Fprintf(os.Stderr, "Fetched %d SHA256 hashes\n", fetchCount)
	}
	return result, nil
}

// fetchHashWithNixPrefetch uses nix store prefetch-file to fetch and hash a file
// This downloads the file and computes the hash, ensuring it's in the Nix store
// Returns the hash in SRI format: sha256-<hash>
func fetchHashWithNixPrefetch(url string) (string, error) {
	trace("Fetching hash with nix store prefetch-file for: %s", url)

	// Run nix store prefetch-file which downloads the file and outputs JSON with the hash
	cmd := exec.Command("nix", "store", "prefetch-file", url, "--json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("nix store prefetch-file failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run nix store prefetch-file: %w", err)
	}

	// Parse JSON output
	var result struct {
		Hash      string `json:"hash"`
		StorePath string `json:"storePath"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse nix output: %w", err)
	}

	if result.Hash == "" {
		return "", fmt.Errorf("nix store prefetch-file returned empty hash")
	}

	// Hash is already in SRI format (sha256-...)
	return result.Hash, nil
}

// fetchHashFromMavenRepo fetches the SHA256 hash from the Maven repository's .sha256 file
// This is faster as it doesn't download the actual JAR file
// Returns the hash in SRI format: sha256-<base64>
func fetchHashFromMavenRepo(url string) (string, error) {
	trace("Fetching hash from Maven repo for: %s", url)

	// Append .sha256 to the URL
	sha256URL := url + ".sha256"

	// Fetch the .sha256 file
	resp, err := http.Get(sha256URL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s: %w", sha256URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch %s: HTTP %d", sha256URL, resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// The file typically contains just the hash, sometimes with whitespace or filename
	// Extract just the hash part (first token)
	hexHash := strings.TrimSpace(string(body))
	// Some repos include the filename after the hash, so take just the first token
	if parts := strings.Fields(hexHash); len(parts) > 0 {
		hexHash = parts[0]
	}

	if hexHash == "" {
		return "", fmt.Errorf("empty hash from %s", sha256URL)
	}

	// Validate it looks like a sha256 hash (64 hex characters)
	if len(hexHash) != 64 {
		return "", fmt.Errorf("invalid hash length from %s: got %d, expected 64", sha256URL, len(hexHash))
	}

	// Convert hex to SRI format (sha256-<base64>)
	sriHash, err := hexToSRI(hexHash)
	if err != nil {
		return "", fmt.Errorf("failed to convert hash to SRI format: %w", err)
	}

	return sriHash, nil
}

// hexToSRI converts a hex-encoded SHA256 hash to SRI format (sha256-<base64>)
func hexToSRI(hexHash string) (string, error) {
	// Decode hex to bytes
	hashBytes, err := hex.DecodeString(hexHash)
	if err != nil {
		return "", fmt.Errorf("invalid hex hash: %w", err)
	}

	// Encode bytes to base64
	base64Hash := base64.StdEncoding.EncodeToString(hashBytes)

	// Return in SRI format
	return "sha256-" + base64Hash, nil
}
