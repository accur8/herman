package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// Global trace mode flag
var traceMode = false

func trace(format string, args ...interface{}) {
	if traceMode {
		fmt.Fprintf(os.Stderr, "[TRACE] "+format+"\n", args...)
	}
}

// getNixSystem returns the Nix system identifier (e.g., "aarch64-darwin", "x86_64-linux")
func getNixSystem() string {
	arch := runtime.GOARCH
	os := runtime.GOOS

	// Map Go arch to Nix arch
	nixArch := arch
	if arch == "amd64" {
		nixArch = "x86_64"
	} else if arch == "arm64" {
		nixArch = "aarch64"
	}

	// Map Go OS to Nix OS
	nixOS := os
	// darwin, linux, etc. are the same in both

	return nixArch + "-" + nixOS
}

type HermanFlags struct {
	Help       bool
	Trace      bool
	Update     bool
	UpdateOnly bool
	Reinstall  bool
	Info       bool
	Version    bool
}

type LauncherConfig struct {
	Kind                      string   `json:"kind,omitempty"`
	MainClass                 string   `json:"mainClass"`
	Organization              string   `json:"organization"`
	Artifact                  string   `json:"artifact"`
	Branch                    string   `json:"branch"`
	JvmArgs                   []string `json:"jvmArgs,omitempty"`
	Args                      []string `json:"args,omitempty"`
	Name                      string   `json:"name,omitempty"`
	Repo                      string   `json:"repo"`
	JavaVersion               string   `json:"javaVersion,omitempty"`
	WebappExplode             *bool    `json:"webappExplode,omitempty"`
	UseNixBuildDescriptionApi *bool    `json:"useNixBuildDescriptionApi,omitempty"`
}

type AppInstallerConfig struct {
	Organization string `json:"organization"`
	Artifact     string `json:"artifact"`
	Version      string `json:"version"`
}

type VersionFile struct {
	Exec               string             `json:"exec"`
	AppInstallerConfig AppInstallerConfig `json:"appInstallerConfig"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Get the program name from the symlink
	programPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	programName := filepath.Base(programPath)
	programDir := filepath.Dir(programPath)

	// Check if we're being called as "herman" directly (command mode)
	if programName == "herman" {
		return runCommandMode()
	}

	// Parse --herman-* flags and filter them out
	hermanFlags, appArgs := parseHermanFlags(os.Args[1:])

	// Enable trace mode if requested
	if hermanFlags.Trace {
		traceMode = true
		trace("Trace mode enabled")
	}

	// Show herman help if requested
	if hermanFlags.Help {
		showHermanHelp()
		return nil
	}

	trace("Program: %s", programName)
	trace("Program dir: %s", programDir)

	// Read the launcher config JSON
	configPath := filepath.Join(programDir, programName+".json")
	trace("Config path: %s", configPath)

	config, err := readLauncherConfig(configPath, programName)
	if err != nil {
		if os.IsNotExist(err) {
			showHelp(programName)
			return nil
		}
		return fmt.Errorf("failed to read launcher config: %w", err)
	}

	trace("Loaded config: %s:%s (org: %s)", config.Artifact, config.Branch, config.Organization)

	// Check if already installed
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	versionFilePath := filepath.Join(homeDir, ".a8", "herman", "builds",
		config.Organization, config.Artifact, fmt.Sprintf("latest-%s", config.Branch), "metadata.json")

	trace("Version file path: %s", versionFilePath)

	// Check if update requested
	needsInstall := false
	var availableVersion string
	var nixBuildResp *NixBuildResponse

	if _, err := os.Stat(versionFilePath); os.IsNotExist(err) {
		trace("Not installed, will install")
		needsInstall = true
	} else if hermanFlags.Update || hermanFlags.UpdateOnly || hermanFlags.Reinstall {
		if hermanFlags.Reinstall {
			trace("Reinstall requested, will force reinstall")
			needsInstall = true
		} else {
			// Smart update: check if version changed
			trace("Update requested, checking for new version")
			var err error
			availableVersion, nixBuildResp, err = checkForUpdates(homeDir, config)
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			// Load current version
			currentVersionFile, err := readVersionFile(versionFilePath)
			if err == nil && currentVersionFile.AppInstallerConfig.Version == availableVersion {
				trace("Already up to date (version: %s)", availableVersion)
				fmt.Fprintf(os.Stderr, "Already up to date: %s\n", availableVersion)
				needsInstall = false
			} else {
				trace("New version available: %s (current: %s)", availableVersion, currentVersionFile.AppInstallerConfig.Version)
				needsInstall = true
			}
		}
	}

	// Install/update if needed
	if needsInstall {
		if err := installWithResponse(homeDir, config, nixBuildResp, availableVersion); err != nil {
			return fmt.Errorf("failed to install: %w", err)
		}
	}

	// Load the version file
	versionFile, err := readVersionFile(versionFilePath)
	if err != nil {
		return fmt.Errorf("failed to read version file: %w", err)
	}

	trace("Loaded version file: %s (version: %s)", versionFile.Exec, versionFile.AppInstallerConfig.Version)

	// Handle --herman-version flag
	if hermanFlags.Version {
		fmt.Printf("Herman launcher for %s\n", programName)
		fmt.Printf("  Organization: %s\n", versionFile.AppInstallerConfig.Organization)
		fmt.Printf("  Artifact:     %s\n", versionFile.AppInstallerConfig.Artifact)
		fmt.Printf("  Version:      %s\n", versionFile.AppInstallerConfig.Version)
		fmt.Printf("  Exec:         %s\n", versionFile.Exec)
		fmt.Println()
	}

	// Handle --herman-info flag
	if hermanFlags.Info {
		buildDir := filepath.Dir(versionFilePath)
		fmt.Printf("Herman Installation Info for %s\n", programName)
		fmt.Printf("  Config:       %s\n", configPath)
		fmt.Printf("  Build dir:    %s\n", buildDir)
		fmt.Printf("  Organization: %s\n", versionFile.AppInstallerConfig.Organization)
		fmt.Printf("  Artifact:     %s\n", versionFile.AppInstallerConfig.Artifact)
		fmt.Printf("  Version:      %s\n", versionFile.AppInstallerConfig.Version)
		fmt.Printf("  Branch:       %s\n", config.Branch)
		fmt.Printf("  Exec:         %s\n", versionFile.Exec)
		fmt.Printf("  Repo:         %s\n", config.Repo)
		fmt.Println()
	}

	// If --herman-update-only, exit now
	if hermanFlags.UpdateOnly {
		fmt.Printf("Updated %s to version %s\n", programName, versionFile.AppInstallerConfig.Version)
		return nil
	}

	// Exec the binary with filtered args (no --herman-* flags)
	trace("Executing: %s with %d args", versionFile.Exec, len(appArgs))
	args := append([]string{versionFile.Exec}, appArgs...)
	if err := syscall.Exec(versionFile.Exec, args, os.Environ()); err != nil {
		return fmt.Errorf("failed to exec %s: %w", versionFile.Exec, err)
	}

	return nil
}

func readLauncherConfig(path string, defaultName string) (*LauncherConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config LauncherConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Apply defaults
	if config.Name == "" {
		if defaultName != "" {
			config.Name = defaultName
		} else {
			config.Name = config.Artifact
		}
	}
	if config.JvmArgs == nil {
		config.JvmArgs = []string{}
	}
	if config.Args == nil {
		config.Args = []string{}
	}
	if config.Repo == "" {
		config.Repo = "repo"
	}

	return &config, nil
}

func readVersionFile(path string) (*VersionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var versionFile VersionFile
	if err := json.Unmarshal(data, &versionFile); err != nil {
		return nil, err
	}

	return &versionFile, nil
}

func showHelp(programName string) {
	fmt.Printf(`Herman - A launcher for Java applications using Nix

USAGE:
  Herman should be invoked via a symlink with a corresponding JSON config file.

SETUP:
  1. Create a symlink to herman:
       ln -s /path/to/herman /usr/local/bin/my-app

  2. Create a config file next to the symlink:
       /usr/local/bin/my-app.json

CONFIG FILE FORMAT:
  {
    "mainClass": "com.example.Main",
    "organization": "com.example",
    "artifact": "my-app_3",
    "branch": "master",
    "jvmArgs": [],
    "args": [],
    "name": "my-app",
    "repo": "repo"
  }

REPOSITORY CONFIG (~/.a8/repo.properties):
  repo_url=https://your-repo.example.com/repos/all
  repo_user=username
  repo_password=password

EXAMPLE:
  $ ln -s /path/to/herman ~/bin/a8-codegen
  $ cat > ~/bin/a8-codegen.json <<EOF
  {
    "mainClass": "a8.codegen.Codegen",
    "organization": "io.accur8",
    "artifact": "a8-codegen_3",
    "branch": "master",
    "jvmArgs": [],
    "args": [],
    "name": "a8-codegen",
    "repo": "repo"
  }
  EOF
  $ a8-codegen --help

CURRENT INVOCATION:
  Program: %s
  Config file expected: %s.json

For more information, see: https://github.com/accur8/herman
`, programName, programName)
}

func parseHermanFlags(args []string) (HermanFlags, []string) {
	flags := HermanFlags{}
	var appArgs []string

	for _, arg := range args {
		switch arg {
		case "--herman-help":
			flags.Help = true
		case "--herman-trace":
			flags.Trace = true
		case "--herman-update":
			flags.Update = true
		case "--herman-update-only":
			flags.UpdateOnly = true
		case "--herman-reinstall":
			flags.Reinstall = true
		case "--herman-info":
			flags.Info = true
		case "--herman-version":
			flags.Version = true
		default:
			// Not a herman flag, pass to app
			appArgs = append(appArgs, arg)
		}
	}

	return flags, appArgs
}

func showHermanHelp() {
	fmt.Printf(`Herman - A launcher for Java applications using Nix

HERMAN FLAGS (for use with symlinked apps):
  --herman-help           Show this help message
  --herman-trace          Enable verbose trace output
  --herman-update         Check for updates (smart - only if version changed), then run
  --herman-update-only    Check for updates, don't run the app
  --herman-reinstall      Force reinstall even if same version
  --herman-version        Show version information, then run
  --herman-info           Show installation information, then run

EXAMPLES:
  # Show Herman help
  a8-codegen --herman-help

  # Run with trace mode
  a8-codegen --herman-trace --help

  # Update to latest version, then run
  a8-codegen --herman-update --help

  # Just update, don't run
  a8-codegen --herman-update-only

  # Show version and installation info
  a8-codegen --herman-version
  a8-codegen --herman-info

  # Combine herman flags with app flags
  a8-codegen --herman-trace --herman-update --some-app-flag

COMMAND MODE (when called as 'herman'):
  herman update <symlink>     Update a specific installation
  herman list                 List all installations
  herman clean <org>/<artifact>  Clean old versions
  herman gc                   Run Nix garbage collection
  herman info <symlink>       Show installation info

For more information: https://github.com/accur8/herman
`)
}

func runCommandMode() error {
	if len(os.Args) < 2 {
		showCommandHelp()
		return nil
	}

	command := os.Args[1]
	switch command {
	case "help", "--help", "-h":
		showCommandHelp()
		return nil
	case "generate":
		return runGenerateCommand()
	default:
		return fmt.Errorf("unknown command: %s\nRun 'herman help' for usage", command)
	}
}

func runGenerateCommand() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("generate command requires a config file path\nUsage: herman generate <config.json> [output-dir] [--dependencies-json <file>] [--trace]")
	}

	configPath := os.Args[2]
	outputDir := "."
	var dependenciesJsonPath string

	// Parse remaining arguments
	for i := 3; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--trace":
			traceMode = true
			trace("Trace mode enabled")
		case "--dependencies-json":
			if i+1 >= len(os.Args) {
				return fmt.Errorf("--dependencies-json requires a file path")
			}
			dependenciesJsonPath = os.Args[i+1]
			i++ // Skip next arg
		default:
			// Assume it's the output directory if we haven't set it yet
			if outputDir == "." && !strings.HasPrefix(arg, "--") {
				outputDir = arg
			}
		}
	}

	// Determine the default name from the output directory or config filename
	defaultName := ""
	if outputDir != "" && outputDir != "." {
		// Use the output directory name as the default name
		defaultName = filepath.Base(outputDir)
	} else {
		// Use the config filename (without .json extension) as the default name
		configBase := filepath.Base(configPath)
		defaultName = strings.TrimSuffix(configBase, ".json")
	}

	// Read the config file
	config, err := readLauncherConfig(configPath, defaultName)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	trace("Loaded config: %s:%s (org: %s)", config.Artifact, config.Branch, config.Organization)

	// Get home directory for repo config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Ensure root flake exists (needed for generated flake.nix to reference)
	if err := ensureRootFlake(homeDir); err != nil {
		return fmt.Errorf("failed to ensure root flake: %w", err)
	}

	var dependencies []Dependency
	var latestVersion string
	var depsVersion string

	// Get repo config (needed for both paths)
	repoConfig, err := readRepoConfig(homeDir, config.Repo)
	if err != nil {
		return fmt.Errorf("failed to read repo config: %w", err)
	}

	// If dependencies.json path is provided, read it directly
	if dependenciesJsonPath != "" {
		trace("Reading dependencies from provided file: %s", dependenciesJsonPath)
		fmt.Fprintf(os.Stderr, "Reading dependencies from %s...\n", dependenciesJsonPath)

		depsJson, err := readDependenciesJsonFile(dependenciesJsonPath)
		if err != nil {
			return fmt.Errorf("failed to read dependencies.json: %w", err)
		}

		dependencies, err = convertDependenciesJsonToDependencies(depsJson, homeDir, repoConfig)
		if err != nil {
			return fmt.Errorf("failed to convert dependencies: %w", err)
		}

		latestVersion = depsJson.Version
		if latestVersion == "" {
			latestVersion = "unknown"
		}

		trace("Loaded %d dependencies from file", len(dependencies))
		fmt.Fprintf(os.Stderr, "Loaded %d dependencies from file\n", len(dependencies))
	} else {
		// Normal flow: fetch maven metadata and get latest version
		fmt.Fprintf(os.Stderr, "Fetching latest version for %s:%s...\n", config.Organization, config.Artifact)

		metadata, err := FetchMavenMetadata(repoConfig, config.Organization, config.Artifact)
		if err != nil {
			return fmt.Errorf("failed to fetch maven metadata: %w", err)
		}

		latestVersion, err = FindLatestVersion(metadata, config.Branch)
		if err != nil {
			return fmt.Errorf("failed to find latest version: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Latest version: %s\n", latestVersion)

		// Try to get dependencies from dependencies.json published in the repo
		trace("Attempting to get dependencies from dependencies.json in repository")
		fmt.Fprintf(os.Stderr, "Fetching dependencies.json from repository...\n")
		dependencies, depsVersion, err = tryGetDependenciesFromJar(repoConfig, homeDir, config.Organization, config.Artifact, latestVersion)

		if err == nil {
			// Successfully got dependencies from dependencies.json
			trace("Successfully got %d dependencies from dependencies.json", len(dependencies))
			fmt.Fprintf(os.Stderr, "Found dependencies.json with %d dependencies\n", len(dependencies))

			// Use the version from dependencies.json if available
			if depsVersion != "" {
				trace("Using version from dependencies.json: %s", depsVersion)
				latestVersion = depsVersion
			}
		} else {
			// dependencies.json fetch or processing failed
			trace("Failed to get dependencies from repository: %v", err)

			// Check if we should fall back to the API
			// Default to false - users must explicitly opt-in to API fallback
			useApi := false
			if config.UseNixBuildDescriptionApi != nil {
				useApi = *config.UseNixBuildDescriptionApi
			}

			if !useApi {
				// Determine if it was a fetch error or conversion error based on the error message
				errMsg := err.Error()
				if strings.Contains(errMsg, "failed to convert dependencies") {
					// dependencies.json was found but had issues
					depsURL := ConstructDependenciesJsonURL(repoConfig.URL, config.Organization, config.Artifact, latestVersion)
					return fmt.Errorf("Found dependencies.json at %s but failed to process it: %w\n\nTo use API fallback instead, set \"useNixBuildDescriptionApi\": true in your config (accepts risks)", depsURL, err)
				} else {
					// dependencies.json not found
					depsURL := ConstructDependenciesJsonURL(repoConfig.URL, config.Organization, config.Artifact, latestVersion)
					return fmt.Errorf("dependencies.json not found at %s\n\nPlease ensure dependencies.json is published alongside the artifact, or set \"useNixBuildDescriptionApi\": true in your config to use the API fallback (accepts risks): %w", depsURL, err)
				}
			}

			// Fall back to API (user has opted in)
			trace("Falling back to nixBuildDescription API (user opted in)")
			fmt.Fprintf(os.Stderr, "dependencies.json not found, falling back to API (useNixBuildDescriptionApi=true)...\n")

			nixBuildResp, err := callNixBuildDescriptionAPIWithVersion(repoConfig, config, config.Name, []string{}, latestVersion)
			if err != nil {
				return fmt.Errorf("failed to fetch dependencies from API: %w", err)
			}

			if len(nixBuildResp.ResolutionResponse.Artifacts) == 0 {
				return fmt.Errorf("no dependencies returned from API")
			}

			dependencies = nixBuildResp.ResolutionResponse.Artifacts
			trace("Received %d dependencies from API", len(dependencies))
		}
	}

	// Fetch missing hashes
	// Use Maven repo .sha256 files for generation (faster, no downloads)
	dependencies, err = FetchMissingHashes(dependencies, false)
	if err != nil {
		return fmt.Errorf("failed to fetch hashes: %w", err)
	}

	// Add the main artifact to the dependencies list
	trace("Adding main artifact %s:%s:%s to dependencies", config.Organization, config.Artifact, latestVersion)
	mainArtifactURL := constructJarURL(repoConfig.URL, config.Organization, config.Artifact, latestVersion)
	trace("Main artifact URL: %s", mainArtifactURL)

	// Fetch hash for main artifact
	mainArtifactHash, err := fetchHashFromMavenRepo(mainArtifactURL)
	if err != nil {
		trace("Failed to fetch hash from .sha256 file, trying nix prefetch: %v", err)
		mainArtifactHash, err = fetchHashWithNixPrefetch(mainArtifactURL)
		if err != nil {
			return fmt.Errorf("failed to fetch hash for main artifact %s:%s: %w", config.Organization, config.Artifact, err)
		}
	}

	orgPath := strings.ReplaceAll(config.Organization, ".", "/")
	mainArtifact := Dependency{
		URL:          mainArtifactURL,
		SHA256:       mainArtifactHash,
		Organization: config.Organization,
		Module:       config.Artifact,
		Version:      latestVersion,
		M2RepoPath:   fmt.Sprintf("%s/%s/%s", orgPath, config.Artifact, latestVersion),
		Filename:     fmt.Sprintf("%s-%s.jar", config.Artifact, latestVersion),
	}

	// Prepend main artifact to dependencies (so it appears first in the list)
	dependencies = append([]Dependency{mainArtifact}, dependencies...)
	trace("Total dependencies including main artifact: %d", len(dependencies))

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate default.nix
	fmt.Fprintf(os.Stderr, "Generating Nix files...\n")

	nixConfig := LauncherNixConfig{
		Name:          config.Name,
		MainClass:     config.MainClass,
		JvmArgs:       config.JvmArgs,
		Args:          config.Args,
		Repo:          config.Repo,
		Organization:  config.Organization,
		Artifact:      config.Artifact,
		Version:       latestVersion,
		Branch:        config.Branch,
		JavaVersion:   config.JavaVersion,
		WebappExplode: config.WebappExplode,
		Dependencies:  dependencies,
	}

	defaultNixContent := GenerateDefaultNix(nixConfig)
	defaultNixPath := filepath.Join(outputDir, "default.nix")
	if err := os.WriteFile(defaultNixPath, []byte(defaultNixContent), 0644); err != nil {
		return fmt.Errorf("failed to write default.nix: %w", err)
	}

	// Create flake.nix that references root Herman flake
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

	flakeNixPath := filepath.Join(outputDir, "flake.nix")
	if err := os.WriteFile(flakeNixPath, []byte(flakeContent), 0644); err != nil {
		return fmt.Errorf("failed to write flake.nix: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Generated Nix files in %s\n", outputDir)
	fmt.Printf("  default.nix\n")
	fmt.Printf("  flake.nix\n")
	fmt.Printf("\nVersion: %s\n", latestVersion)
	fmt.Printf("Dependencies: %d\n", len(dependencies))

	return nil
}

func showCommandHelp() {
	fmt.Printf(`Herman - A launcher for Java applications using Nix

USAGE:
  herman <command> [arguments]

COMMANDS:
  help                        Show this help message
  generate <config.json> [output-dir] [--dependencies-json <file>] [--trace]
                              Generate Nix files from config (for embedding in Nix builds)
                              Options:
                                --dependencies-json <file>  Use dependencies from file (for testing)
                                --trace                     Enable verbose trace output
  update <symlink>            Update a specific installation
  list                        List all installations
  clean <org>/<artifact>      Clean old versions
  gc                          Run Nix garbage collection
  info <symlink>              Show installation info

EXAMPLES:
  # Generate from jar (normal use)
  herman generate my-app.json ./nix-output

  # Generate from dependencies.json file (testing/offline)
  herman generate my-app.json ./nix-output --dependencies-json deps.json

  # Other commands
  herman update ~/bin/a8-codegen
  herman list
  herman clean io.accur8/a8-versions_3
  herman info ~/bin/a8-codegen

FLAGS IN LAUNCHER MODE:
  When using Herman via a symlink, you can use --herman-* flags:

  a8-codegen --herman-help
  a8-codegen --herman-trace
  a8-codegen --herman-update
  a8-codegen --herman-version
  a8-codegen --herman-info

For more information: https://github.com/accur8/herman
`)
}
