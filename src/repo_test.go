package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadRepoConfig(t *testing.T) {
	// Create a temp directory for test files
	tempDir, err := os.MkdirTemp("", "herman-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test repo.properties file
	a8Dir := filepath.Join(tempDir, ".a8")
	if err := os.MkdirAll(a8Dir, 0755); err != nil {
		t.Fatalf("Failed to create .a8 dir: %v", err)
	}

	repoProps := `# Test repo properties
repo_url=https://locus2.accur8.net/repos/all
repo_user=reader
repo_password=a_password

bob_url=https://bob.example.com/repos
bob_user=bob_user
bob_password=bob_pass

minimal_url=https://minimal.example.com/repos
`

	repoPropsPath := filepath.Join(a8Dir, "repo.properties")
	if err := os.WriteFile(repoPropsPath, []byte(repoProps), 0644); err != nil {
		t.Fatalf("Failed to write repo.properties: %v", err)
	}

	// Test reading "repo" config
	t.Run("ReadRepoConfig", func(t *testing.T) {
		config, err := readRepoConfig(tempDir, "repo")
		if err != nil {
			t.Fatalf("Failed to read repo config: %v", err)
		}

		if config.URL != "https://locus2.accur8.net/repos/all" {
			t.Errorf("Expected URL 'https://locus2.accur8.net/repos/all', got '%s'", config.URL)
		}
		if config.User != "reader" {
			t.Errorf("Expected user 'reader', got '%s'", config.User)
		}
		if config.Password != "a_password" {
			t.Errorf("Expected password 'a_password', got '%s'", config.Password)
		}
	})

	// Test reading "bob" config
	t.Run("ReadBobConfig", func(t *testing.T) {
		config, err := readRepoConfig(tempDir, "bob")
		if err != nil {
			t.Fatalf("Failed to read bob config: %v", err)
		}

		if config.URL != "https://bob.example.com/repos" {
			t.Errorf("Expected URL 'https://bob.example.com/repos', got '%s'", config.URL)
		}
		if config.User != "bob_user" {
			t.Errorf("Expected user 'bob_user', got '%s'", config.User)
		}
		if config.Password != "bob_pass" {
			t.Errorf("Expected password 'bob_pass', got '%s'", config.Password)
		}
	})

	// Test reading config with optional credentials
	t.Run("ReadMinimalConfig", func(t *testing.T) {
		config, err := readRepoConfig(tempDir, "minimal")
		if err != nil {
			t.Fatalf("Failed to read minimal config: %v", err)
		}

		if config.URL != "https://minimal.example.com/repos" {
			t.Errorf("Expected URL 'https://minimal.example.com/repos', got '%s'", config.URL)
		}
		if config.User != "" {
			t.Errorf("Expected empty user, got '%s'", config.User)
		}
		if config.Password != "" {
			t.Errorf("Expected empty password, got '%s'", config.Password)
		}
	})

	// Test error case - missing repo
	t.Run("MissingRepo", func(t *testing.T) {
		_, err := readRepoConfig(tempDir, "nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent repo, got nil")
		}
	})
}
