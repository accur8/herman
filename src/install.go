package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

// getNixpkgsURL returns the appropriate nixpkgs URL based on the target environment
func getNixpkgsURL() string {
	// macOS → github:NixOS/nixpkgs
	if runtime.GOOS == "darwin" {
		return "github:NixOS/nixpkgs"
	}

	// Linux: check if NixOS
	if runtime.GOOS == "linux" {
		// Check for NixOS by looking for /etc/NIXOS
		if _, err := os.Stat("/etc/NIXOS"); err == nil {
			// NixOS → github:NixOS/nixpkgs/nixos-unstable
			return "github:NixOS/nixpkgs/nixos-unstable"
		}
		// non-NixOS Linux → github:NixOS/nixpkgs
		return "github:NixOS/nixpkgs"
	}

	// Fallback for other systems
	return "github:NixOS/nixpkgs"
}

// ensureRootFlake ensures that the root flake.nix exists at ~/.a8/herman/
func ensureRootFlake(homeDir string) error {
	hermanRootDir := filepath.Join(homeDir, ".a8", "herman")
	flakeNixPath := filepath.Join(hermanRootDir, "flake.nix")

	// Check if flake.nix already exists
	if _, err := os.Stat(flakeNixPath); err == nil {
		trace("Root flake already exists at %s", flakeNixPath)
		return nil
	}

	trace("Creating root flake at %s", flakeNixPath)

	// Ensure the herman directory exists
	if err := os.MkdirAll(hermanRootDir, 0755); err != nil {
		return fmt.Errorf("failed to create herman directory: %w", err)
	}

	// Get the appropriate nixpkgs URL for this environment
	nixpkgsURL := getNixpkgsURL()
	trace("Using nixpkgs URL: %s", nixpkgsURL)

	// Create the root flake.nix
	flakeContent := fmt.Sprintf(`{
  description = "Herman - shared nixpkgs for all managed packages";

  inputs = {
    nixpkgs.url = "%s";
  };

  outputs = { self, nixpkgs }: {
    # This flake provides a shared nixpkgs input for all Herman-managed packages
    # Update with: nix flake update
  };
}
`, nixpkgsURL)

	if err := os.WriteFile(flakeNixPath, []byte(flakeContent), 0644); err != nil {
		return fmt.Errorf("failed to write root flake.nix: %w", err)
	}

	// Initialize the flake.lock
	fmt.Fprintf(os.Stderr, "Initializing shared nixpkgs (this may take a moment)...\n")
	cmd := exec.Command("nix", "flake", "lock")
	cmd.Dir = hermanRootDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize flake.lock: %w", err)
	}

	trace("Root flake initialized successfully")
	fmt.Fprintf(os.Stderr, "Shared nixpkgs initialized at %s\n", hermanRootDir)

	return nil
}

// checkForUpdates uses maven-metadata.xml to find the latest version for the branch
func checkForUpdates(homeDir string, config *LauncherConfig) (string, *NixBuildResponse, error) {
	// Get repo config
	repoConfig, err := readRepoConfig(homeDir, config.Repo)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read repo config: %w", err)
	}

	// Fetch maven-metadata.xml to find available versions
	trace("Fetching maven-metadata.xml for %s:%s", config.Organization, config.Artifact)
	fmt.Fprintf(os.Stderr, "Checking for latest version from %s...\n", repoConfig.URL)
	metadata, err := FetchMavenMetadata(repoConfig, config.Organization, config.Artifact)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch maven-metadata.xml: %w", err)
	}

	// Find the latest version for the configured branch
	trace("Finding latest version for branch: %s", config.Branch)
	latestVersion, err := FindLatestVersion(metadata, config.Branch)
	if err != nil {
		return "", nil, fmt.Errorf("failed to find latest version: %w", err)
	}

	trace("Latest version for branch %s: %s", config.Branch, latestVersion)
	fmt.Fprintf(os.Stderr, "Latest version: %s\n", latestVersion)

	// Call the API with the explicit version to get dependencies
	programPath, err := os.Executable()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	programName := filepath.Join(filepath.Dir(programPath), filepath.Base(programPath))

	trace("Fetching dependencies for version %s", latestVersion)
	fmt.Fprintf(os.Stderr, "Fetching dependencies...\n")
	nixBuildResp, err := callNixBuildDescriptionAPIWithVersion(repoConfig, config, programName, os.Args, latestVersion)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get dependencies: %w", err)
	}

	// Check if we got dependencies in the response
	if len(nixBuildResp.ResolutionResponse.Artifacts) == 0 {
		// Fall back to extracting from files if available (backward compatibility)
		if len(nixBuildResp.Files) > 0 {
			trace("No artifacts in resolutionResponse, falling back to files (old API)")
			version, err := extractVersionFromNixFiles(nixBuildResp.Files)
			if err != nil {
				return "", nil, fmt.Errorf("failed to extract version: %w", err)
			}
			return version, nixBuildResp, nil
		}
		return "", nil, fmt.Errorf("no dependencies found in API response")
	}

	trace("Received %d dependencies from API", len(nixBuildResp.ResolutionResponse.Artifacts))
	if len(nixBuildResp.Files) > 0 {
		trace("Note: API also returned %d files (will be ignored, using local generation)", len(nixBuildResp.Files))
	} else {
		trace("API correctly returned only dependency resolution (no files)")
	}

	return latestVersion, nixBuildResp, nil
}

func install(homeDir string, config *LauncherConfig) error {
	return installWithResponse(homeDir, config, nil, "")
}

func installWithResponse(homeDir string, config *LauncherConfig, nixBuildResp *NixBuildResponse, version string) error {
	// Ensure root flake exists
	if err := ensureRootFlake(homeDir); err != nil {
		return fmt.Errorf("failed to ensure root flake: %w", err)
	}

	// If we don't have the API response yet, fetch it
	if nixBuildResp == nil {
		var err error
		version, nixBuildResp, err = checkForUpdates(homeDir, config)
		if err != nil {
			return err
		}
	}

	// Version is either passed as parameter or obtained from checkForUpdates
	trace("Using version: %s", version)

	// Create the version-specific build directory early
	buildDir := filepath.Join(homeDir, ".a8", "herman", "builds", config.Organization, config.Artifact)
	versionDir := filepath.Join(buildDir, version)
	nixBuildDir := filepath.Join(versionDir, "nix-build")
	trace("Creating version directory: %s", versionDir)
	if err := os.MkdirAll(nixBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create nix-build directory: %w", err)
	}

	// Save the raw API response for debugging
	apiResponsePath := filepath.Join(nixBuildDir, "nixBuildDescription-response.json")
	apiResponseJSON, err := json.MarshalIndent(nixBuildResp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal API response: %w", err)
	}
	if err := os.WriteFile(apiResponsePath, apiResponseJSON, 0644); err != nil {
		return fmt.Errorf("failed to write API response: %w", err)
	}
	trace("Saved API response to %s", apiResponsePath)

	// Determine which dependencies to use
	var dependencies []Dependency
	if len(nixBuildResp.ResolutionResponse.Artifacts) > 0 {
		// Use dependencies from resolutionResponse (preferred)
		trace("Using %d dependencies from resolutionResponse", len(nixBuildResp.ResolutionResponse.Artifacts))
		dependencies = nixBuildResp.ResolutionResponse.Artifacts

		// Fetch missing hashes if needed
		// Use nix-prefetch-url for installation to ensure files are in Nix store
		var err error
		dependencies, err = FetchMissingHashes(dependencies, true)
		if err != nil {
			return fmt.Errorf("failed to fetch missing hashes: %w", err)
		}
	} else if len(nixBuildResp.Files) > 0 {
		// Fall back to writing files from API (backward compatibility)
		trace("No artifacts in resolutionResponse, falling back to files from API")
		fmt.Fprintf(os.Stderr, "Writing %d files to %s...\n", len(nixBuildResp.Files), nixBuildDir)
		for _, file := range nixBuildResp.Files {
			filePath := filepath.Join(nixBuildDir, file.Filename)

			// Create parent directory if needed
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", file.Filename, err)
			}

			if err := os.WriteFile(filePath, []byte(file.Contents), 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", file.Filename, err)
			}
		}
		// Skip to flake.nix creation
		dependencies = nil
	} else {
		return fmt.Errorf("no dependencies or files in API response")
	}

	// If we have dependencies, generate Nix files locally
	if len(dependencies) > 0 {
		trace("Generating Nix files locally from %d dependencies", len(dependencies))
		fmt.Fprintf(os.Stderr, "Generating Nix files from dependency tree...\n")

		// Determine Java version from config (if any)
		javaVersion := ""
		// TODO: Add logic to detect Java version from launcher config if needed

		// Generate default.nix
		nixConfig := LauncherNixConfig{
			Name:          config.Name,
			MainClass:     config.MainClass,
			JvmArgs:       config.JvmArgs,
			Args:          config.Args,
			Repo:          config.Repo,
			Organization:  config.Organization,
			Artifact:      config.Artifact,
			Version:       version,
			Branch:        config.Branch,
			JavaVersion:   javaVersion,
			WebappExplode: config.WebappExplode,
			Dependencies:  dependencies,
		}

		defaultNixContent := GenerateDefaultNix(nixConfig)
		defaultNixPath := filepath.Join(nixBuildDir, "default.nix")
		if err := os.WriteFile(defaultNixPath, []byte(defaultNixContent), 0644); err != nil {
			return fmt.Errorf("failed to write default.nix: %w", err)
		}
		trace("Generated default.nix")

		fmt.Fprintf(os.Stderr, "Generated Nix build files\n")
	}

	// Create the per-package flake.nix that follows the root flake
	flakeNixPath := filepath.Join(nixBuildDir, "flake.nix")
	nixSystem := getNixSystem()
	hermanRootPath := filepath.Join(homeDir, ".a8", "herman")

	flakeContent := fmt.Sprintf(`{
  description = "%s - managed by Herman";

  inputs = {
    hermanRoot.url = "path:%s";
    nixpkgs.follows = "hermanRoot/nixpkgs";
  };

  outputs = { self, nixpkgs, hermanRoot }:
    let
      system = "%s";
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      packages.${system}.default = pkgs.callPackage ./default.nix {};
    };
}
`, config.Name, hermanRootPath, nixSystem)

	if err := os.WriteFile(flakeNixPath, []byte(flakeContent), 0644); err != nil {
		return fmt.Errorf("failed to write flake.nix: %w", err)
	}
	trace("Created flake.nix for %s", config.Name)

	// Create build.sh script showing the exact nix build invocation
	buildScript := filepath.Join(nixBuildDir, "build.sh")
	buildScriptContent := fmt.Sprintf(`#!/usr/bin/env bash
# Herman nix build invocation
# Organization: %s
# Artifact: %s
# Version: %s
# Date: %s
# Nixpkgs: shared via %s/flake.lock

set -e

nix build --out-link result
`, config.Organization, config.Artifact, version, time.Now().Format(time.RFC3339), hermanRootPath)

	if err := os.WriteFile(buildScript, []byte(buildScriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write build script: %w", err)
	}
	trace("Created build script: %s", buildScript)

	// Run nix build from the permanent directory
	trace("Running nix build in %s", nixBuildDir)
	fmt.Fprintf(os.Stderr, "Running nix build (using shared nixpkgs)...\n")
	cmd := exec.Command("nix", "build", "--out-link", "result")
	cmd.Dir = nixBuildDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nix build failed: %w", err)
	}
	trace("nix build completed successfully")

	// Resolve the result symlink to get the canonical path
	resultLink := filepath.Join(nixBuildDir, "result")
	nixStorePath, err := filepath.EvalSymlinks(resultLink)
	if err != nil {
		return fmt.Errorf("failed to resolve build symlink: %w", err)
	}
	trace("Resolved nix store path: %s", nixStorePath)

	// Find the exec path in the bin directory
	nixStoreBinPath := filepath.Join(nixStorePath, "bin", "launch")
	if _, err := os.Stat(nixStoreBinPath); os.IsNotExist(err) {
		// If "launch" doesn't exist, try to find the first executable
		binDir := filepath.Join(nixStorePath, "bin")
		entries, err := os.ReadDir(binDir)
		if err != nil {
			return fmt.Errorf("failed to read bin directory: %w", err)
		}
		if len(entries) > 0 {
			nixStoreBinPath = filepath.Join(binDir, entries[0].Name())
		} else {
			return fmt.Errorf("no executables found in %s", binDir)
		}
	}

	// Create a symlink in the version directory pointing to the nix store binary
	execSymlink := filepath.Join(versionDir, config.Name)
	os.Remove(execSymlink) // Remove existing symlink if it exists
	trace("Creating exec symlink: %s -> %s", execSymlink, nixStoreBinPath)
	if err := os.Symlink(nixStoreBinPath, execSymlink); err != nil {
		return fmt.Errorf("failed to create exec symlink: %w", err)
	}

	// Create the version file structure
	versionFile := VersionFile{
		Exec: execSymlink,
		AppInstallerConfig: AppInstallerConfig{
			Organization: config.Organization,
			Artifact:     config.Artifact,
			Version:      version,
		},
	}

	// Write metadata.json in version directory
	metadataPath := filepath.Join(versionDir, "metadata.json")
	versionJSON, err := json.MarshalIndent(versionFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, versionJSON, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	// Create the latest-<branch> symlink pointing to version directory
	latestSymlink := filepath.Join(buildDir, fmt.Sprintf("latest-%s", config.Branch))
	os.Remove(latestSymlink) // Remove existing symlink if it exists
	trace("Creating latest symlink: %s -> %s", latestSymlink, version)
	if err := os.Symlink(version, latestSymlink); err != nil {
		return fmt.Errorf("failed to create latest symlink: %w", err)
	}

	trace("Installation complete")
	fmt.Fprintf(os.Stderr, "Installation complete: %s\n", execSymlink)
	return nil
}

func extractVersionFromNixFiles(files []NixFile) (string, error) {
	// Look for the version in the default.nix file
	for _, file := range files {
		if file.Filename == "default.nix" {
			// Try to extract version from the contents
			// Look for pattern like: version = "Version(1.0.0-20241022_1519_master)"
			versionRegex := regexp.MustCompile(`version\s*=\s*"Version\(([^)]+)\)"`)
			matches := versionRegex.FindStringSubmatch(file.Contents)
			if len(matches) > 1 {
				return matches[1], nil
			}

			// Alternative: look for version = Version(...) in dependencies
			versionRegex2 := regexp.MustCompile(`version\s*=\s*"Version\(([0-9].*?)\)"`)
			matches2 := versionRegex2.FindStringSubmatch(file.Contents)
			if len(matches2) > 1 {
				return matches2[1], nil
			}
		}
	}

	return "unknown", nil
}
