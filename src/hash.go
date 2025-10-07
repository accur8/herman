package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// FetchMissingHashes fetches SHA256 hashes for dependencies that don't have them
// Uses nix-prefetch-url in parallel for efficiency
func FetchMissingHashes(dependencies []Dependency) ([]Dependency, error) {
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

				hash, err := fetchHashWithNixPrefetch(url)
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

// fetchHashWithNixPrefetch uses nix-prefetch-url to fetch and hash a file
func fetchHashWithNixPrefetch(url string) (string, error) {
	trace("Fetching hash for: %s", url)

	// Run nix-prefetch-url which downloads the file and outputs the hash
	cmd := exec.Command("nix-prefetch-url", url)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("nix-prefetch-url failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run nix-prefetch-url: %w", err)
	}

	// The output is the hash (with a newline)
	hash := strings.TrimSpace(string(output))

	if hash == "" {
		return "", fmt.Errorf("nix-prefetch-url returned empty hash")
	}

	return hash, nil
}
