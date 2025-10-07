package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadLauncherConfig(t *testing.T) {
	// Create a temp directory for test files
	tempDir, err := os.MkdirTemp("", "herman-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test launcher config
	config := LauncherConfig{
		MainClass:    "a8.codegen.Codegen",
		Organization: "io.accur8",
		Artifact:     "a8-versions_3",
		Branch:       "master",
		JvmArgs:      []string{"-Xmx512m"},
		Args:         []string{"--verbose"},
		Name:         "a8-codegen",
		Repo:         "repo",
	}

	configPath := filepath.Join(tempDir, "test-launcher.json")
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test reading the config
	t.Run("ReadLauncherConfig", func(t *testing.T) {
		readConfig, err := readLauncherConfig(configPath)
		if err != nil {
			t.Fatalf("Failed to read launcher config: %v", err)
		}

		if readConfig.MainClass != config.MainClass {
			t.Errorf("Expected MainClass '%s', got '%s'", config.MainClass, readConfig.MainClass)
		}
		if readConfig.Organization != config.Organization {
			t.Errorf("Expected Organization '%s', got '%s'", config.Organization, readConfig.Organization)
		}
		if readConfig.Artifact != config.Artifact {
			t.Errorf("Expected Artifact '%s', got '%s'", config.Artifact, readConfig.Artifact)
		}
		if readConfig.Branch != config.Branch {
			t.Errorf("Expected Branch '%s', got '%s'", config.Branch, readConfig.Branch)
		}
		if len(readConfig.JvmArgs) != 1 || readConfig.JvmArgs[0] != "-Xmx512m" {
			t.Errorf("Expected JvmArgs ['-Xmx512m'], got %v", readConfig.JvmArgs)
		}
		if len(readConfig.Args) != 1 || readConfig.Args[0] != "--verbose" {
			t.Errorf("Expected Args ['--verbose'], got %v", readConfig.Args)
		}
	})

	// Test error case - missing file
	t.Run("MissingFile", func(t *testing.T) {
		_, err := readLauncherConfig(filepath.Join(tempDir, "nonexistent.json"))
		if err == nil {
			t.Error("Expected error for missing file, got nil")
		}
	})
}

func TestReadVersionFile(t *testing.T) {
	// Create a temp directory for test files
	tempDir, err := os.MkdirTemp("", "herman-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test version file
	versionFile := VersionFile{
		Exec: "/nix/store/abc123-test/bin/launch",
		AppInstallerConfig: AppInstallerConfig{
			Organization: "io.accur8",
			Artifact:     "a8-codegen_3",
			Version:      "1.0.0-20241022_1519_master",
		},
	}

	versionPath := filepath.Join(tempDir, "latest_master.json")
	versionJSON, err := json.MarshalIndent(versionFile, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal version file: %v", err)
	}

	if err := os.WriteFile(versionPath, versionJSON, 0644); err != nil {
		t.Fatalf("Failed to write version file: %v", err)
	}

	// Test reading the version file
	t.Run("ReadVersionFile", func(t *testing.T) {
		readVersion, err := readVersionFile(versionPath)
		if err != nil {
			t.Fatalf("Failed to read version file: %v", err)
		}

		if readVersion.Exec != versionFile.Exec {
			t.Errorf("Expected Exec '%s', got '%s'", versionFile.Exec, readVersion.Exec)
		}
		if readVersion.AppInstallerConfig.Organization != versionFile.AppInstallerConfig.Organization {
			t.Errorf("Expected Organization '%s', got '%s'",
				versionFile.AppInstallerConfig.Organization,
				readVersion.AppInstallerConfig.Organization)
		}
		if readVersion.AppInstallerConfig.Version != versionFile.AppInstallerConfig.Version {
			t.Errorf("Expected Version '%s', got '%s'",
				versionFile.AppInstallerConfig.Version,
				readVersion.AppInstallerConfig.Version)
		}
	})
}
