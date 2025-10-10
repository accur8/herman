package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRegressions(t *testing.T) {
	// Find the test runner script
	// When running from src directory, go up one level to repo root
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}
	testRunner := filepath.Join(repoRoot, "test", "unit", "run_tests.sh")

	// Check if test runner exists
	if _, err := os.Stat(testRunner); os.IsNotExist(err) {
		t.Skipf("Test runner not found at %s, skipping regression tests", testRunner)
	}

	// Build herman first if result/bin/herman doesn't exist
	hermanBinary := filepath.Join(repoRoot, "result", "bin", "herman")
	if _, err := os.Stat(hermanBinary); os.IsNotExist(err) {
		t.Log("Herman binary not found, attempting to build with nix...")
		buildCmd := exec.Command("nix", "build")
		buildCmd.Dir = repoRoot
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to build herman: %v\nOutput: %s", err, buildOutput)
		}
	}

	// Run the test runner script
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("bash", testRunner)
	cmd.Dir = repoRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HERMAN="+hermanBinary)

	err = cmd.Run()

	// Always show output
	if stdout.Len() > 0 {
		t.Logf("Test output:\n%s", stdout.String())
	}
	if stderr.Len() > 0 {
		t.Logf("Test errors:\n%s", stderr.String())
	}

	// Check if tests passed
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Regression tests failed with exit code %d", exitErr.ExitCode())
		}
		t.Fatalf("Failed to run regression tests: %v", err)
	}

	// Parse output to count tests
	output := stdout.String()
	if strings.Contains(output, "All tests passed!") {
		t.Log("All regression tests passed")
	}
}
