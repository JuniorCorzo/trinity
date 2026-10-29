package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLI_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-v"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for -v, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "dev-tui v") {
		t.Errorf("expected version string in output, got: %s", out)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for --version, got %d", code)
	}
	if !strings.Contains(stdout.String(), "dev-tui v") {
		t.Errorf("expected version string in output, got: %s", stdout.String())
	}
}

func TestCLI_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for -h, got %d", code)
	}
	out := stderr.String()
	if !strings.Contains(out, "KEYBOARD SHORTCUTS") || !strings.Contains(out, "CONFIGURATION") {
		t.Errorf("expected help output with keyboard shortcuts and configuration, got: %s", out)
	}
}

func TestCLI_InvalidFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--unknown-flag"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for invalid flag, got %d", code)
	}
}

func TestCLI_NonExistentConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-config", "/nonexistent/file.json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 for non-existent config, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Error loading configuration") {
		t.Errorf("expected error loading configuration in stderr, got: %s", stderr.String())
	}
}

func TestCLI_EmptyDirNoServices(t *testing.T) {
	emptyDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", emptyDir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 for empty directory with no services, got %d", code)
	}
	if !strings.Contains(stderr.String(), "No services found") {
		t.Errorf("expected 'No services found' message in stderr, got: %s", stderr.String())
	}
}

func TestCLI_Init(t *testing.T) {
	tmpDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"init", tmpDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for init, got %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Created configuration file") {
		t.Errorf("expected confirmation message in stdout, got: %s", stdout.String())
	}

	// Running init again without force should return error
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"-dir", tmpDir, "--init"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 when re-running init without force, got %d", code)
	}

	// Running init with force should succeed
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"-dir", tmpDir, "--init", "--force"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 when re-running init with force, got %d", code)
	}
}

func TestCLI_ThemeList(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"theme", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for 'theme list', got %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "terminal") || !strings.Contains(out, "catppuccin-noir") {
		t.Errorf("expected theme list to contain 'terminal' and 'catppuccin-noir', got: %s", out)
	}
}
