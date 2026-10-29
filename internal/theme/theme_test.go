package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTheme_Builtin(t *testing.T) {
	tests := []struct {
		name        string
		wantName    string
		wantTransp  bool
	}{
		{"", "terminal", true},
		{"default", "terminal", true},
		{"terminal", "terminal", true},
		{"catppuccin-noir", "catppuccin-noir", false},
		{"dracula", "dracula", false},
		{"tokyo-night", "tokyo-night", false},
	}

	for _, tt := range tests {
		th, err := LoadTheme(tt.name, "")
		if err != nil {
			t.Fatalf("LoadTheme(%q) unexpected error: %v", tt.name, err)
		}
		if th.Name != tt.wantName {
			t.Errorf("LoadTheme(%q).Name = %q, want %q", tt.name, th.Name, tt.wantName)
		}
		if th.Transparent != tt.wantTransp {
			t.Errorf("LoadTheme(%q).Transparent = %v, want %v", tt.name, th.Transparent, tt.wantTransp)
		}
	}
}

func TestLoadTheme_CustomDir(t *testing.T) {
	tempDir := t.TempDir()
	customDir := filepath.Join(tempDir, ".trinity", "themes")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}

	custom := Theme{
		Name:        "nord",
		Description: "Arctic ice theme",
		Transparent: false,
		Bg:          "#2e3440",
		Primary:     "#88c0d0",
	}
	data, _ := json.Marshal(custom)
	if err := os.WriteFile(filepath.Join(customDir, "nord.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	th, err := LoadTheme("nord", tempDir)
	if err != nil {
		t.Fatalf("LoadTheme('nord') error: %v", err)
	}
	if th.Name != "nord" || th.Bg != "#2e3440" {
		t.Errorf("unexpected theme: %+v", th)
	}
}

func TestInstallTheme_LocalFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	srcFile := filepath.Join(tempDir, "solarized.json")
	solarized := Theme{
		Name:        "solarized-dark",
		Description: "Classic solarized dark",
		Transparent: false,
		Bg:          "#002b36",
		Primary:     "#268bd2",
	}
	data, _ := json.Marshal(solarized)
	if err := os.WriteFile(srcFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	th, dest, err := InstallTheme(srcFile)
	if err != nil {
		t.Fatalf("InstallTheme error: %v", err)
	}
	if th.Name != "solarized-dark" {
		t.Errorf("installed theme name = %q, want solarized-dark", th.Name)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("installed file does not exist at %s: %v", dest, err)
	}

	// Verify it can now be loaded
	loaded, err := LoadTheme("solarized-dark", "")
	if err != nil {
		t.Fatalf("failed to load newly installed theme: %v", err)
	}
	if loaded.Bg != "#002b36" {
		t.Errorf("loaded theme bg = %q, want #002b36", loaded.Bg)
	}
}

func TestListThemes(t *testing.T) {
	themes := ListThemes("")
	if len(themes) < 4 {
		t.Errorf("expected at least 4 built-in themes, got %d", len(themes))
	}
	if themes[0].Name != "terminal" {
		t.Errorf("expected first theme to be 'terminal', got %q", themes[0].Name)
	}
}
