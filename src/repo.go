package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RepoConfig struct {
	URL      string
	User     string
	Password string
}

func readRepoConfig(homeDir, repoName string) (*RepoConfig, error) {
	repoPropsPath := filepath.Join(homeDir, ".a8", "repo.properties")

	file, err := os.Open(repoPropsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repo.properties: %w", err)
	}
	defer file.Close()

	props := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			props[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read repo.properties: %w", err)
	}

	// Look for repo-specific config
	urlKey := repoName + "_url"
	userKey := repoName + "_user"
	passwordKey := repoName + "_password"

	url, ok := props[urlKey]
	if !ok {
		return nil, fmt.Errorf("repo URL not found for '%s' (looking for %s)", repoName, urlKey)
	}

	config := &RepoConfig{
		URL:      url,
		User:     props[userKey],     // Optional
		Password: props[passwordKey], // Optional
	}

	return config, nil
}
