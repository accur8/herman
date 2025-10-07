package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type NixBuildRequest struct {
	Kind                string                 `json:"kind"`
	MainClass           string                 `json:"mainClass"`
	Organization        string                 `json:"organization"`
	Artifact            string                 `json:"artifact"`
	Branch              string                 `json:"branch"`
	CommandLineParms    map[string]interface{} `json:"commandLineParms"`
	Quiet               bool                   `json:"quiet"`
	LogRollers          []interface{}          `json:"logRollers"`
	LogFiles            bool                   `json:"logFiles"`
	InstallDir          interface{}            `json:"installDir"`
	JvmArgs             []string               `json:"jvmArgs"`
	Args                []string               `json:"args"`
	Name                string                 `json:"name"`
	Repo                string                 `json:"repo"`
}

type NixFile struct {
	Filename string `json:"filename"`
	Contents string `json:"contents"`
}

type NixBuildResponse struct {
	Files              []NixFile          `json:"files"`
	ResolutionResponse ResolutionResponse `json:"resolutionResponse"`
}

type ResolutionResponse struct {
	Artifacts []Dependency `json:"artifacts"`
}

func callNixBuildDescriptionAPI(repoConfig *RepoConfig, config *LauncherConfig, programName string, args []string) (*NixBuildResponse, error) {
	return callNixBuildDescriptionAPIWithVersion(repoConfig, config, programName, args, "")
}

func callNixBuildDescriptionAPIWithVersion(repoConfig *RepoConfig, config *LauncherConfig, programName string, args []string, explicitVersion string) (*NixBuildResponse, error) {
	// Build the API endpoint URL
	apiURL := strings.TrimRight(repoConfig.URL, "/")
	// Replace /repos/all with /api/nixBuildDescription
	apiURL = strings.Replace(apiURL, "/repos/all", "/api/nixBuildDescription", 1)
	trace("API URL: %s", apiURL)

	// Set explicitVersion if provided
	var explicitVersionParam interface{} = nil
	if explicitVersion != "" {
		explicitVersionParam = explicitVersion
		trace("Using explicit version: %s", explicitVersion)
	}

	// Build the request
	// Note: resolveOnly=true tells the server we only need dependency resolution,
	// not the full Nix file generation (we generate those locally)
	request := NixBuildRequest{
		Kind:         "jvm_cli",
		MainClass:    config.MainClass,
		Organization: config.Organization,
		Artifact:     config.Artifact,
		Branch:       config.Branch,
		CommandLineParms: map[string]interface{}{
			"programName":            programName,
			"rawCommandLineArgs":     args,
			"resolvedCommandLineArgs": []string{},
			"resolveOnly":            true, // Request only dependency resolution, not full Nix files
			"quiet":                  false,
			"explicitVersion":        explicitVersionParam,
			"launcherJson":           nil,
			"showHelp":               false,
		},
		Quiet:      false,
		LogRollers: []interface{}{},
		LogFiles:   false,
		InstallDir: nil,
		JvmArgs:    config.JvmArgs,
		Args:       config.Args,
		Name:       config.Name,
		Repo:       config.Repo,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	trace("Request body size: %d bytes", len(jsonData))

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add basic auth if credentials are provided
	if repoConfig.User != "" && repoConfig.Password != "" {
		trace("Using basic auth with user: %s", repoConfig.User)
		req.SetBasicAuth(repoConfig.User, repoConfig.Password)
	}

	// Send the request
	trace("Sending POST request to API")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	trace("Received response with status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		switch resp.StatusCode {
		case http.StatusNotFound:
			return nil, fmt.Errorf("artifact not found: %s:%s (organization: %s)\nCheck that the artifact name and organization are correct in your config JSON",
				config.Artifact, config.Branch, config.Organization)
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, fmt.Errorf("authentication failed: check credentials in ~/.a8/repo.properties for repo '%s'", config.Repo)
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
			// Don't dump the whole stack trace to the user
			bodyStr := string(body)
			if len(bodyStr) > 500 {
				bodyStr = bodyStr[:500] + "..."
			}
			return nil, fmt.Errorf("server error (status %d): the artifact may not exist or there may be a repository issue\nArtifact: %s:%s\nOrganization: %s\nFirst 500 chars of error: %s",
				resp.StatusCode, config.Artifact, config.Branch, config.Organization, bodyStr)
		default:
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}
	}

	// Parse the response
	var nixBuildResp NixBuildResponse
	if err := json.NewDecoder(resp.Body).Decode(&nixBuildResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &nixBuildResp, nil
}
