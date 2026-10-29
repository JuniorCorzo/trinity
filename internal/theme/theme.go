package theme

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Theme holds color definitions and transparency flag for the TUI.
type Theme struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Transparent  bool   `json:"transparent"`
	Bg           string `json:"bg"`
	Surface      string `json:"surface"`
	CardBg       string `json:"card_bg"`
	Border       string `json:"border"`
	BorderActive string `json:"border_active"`
	Primary      string `json:"primary"`
	Secondary    string `json:"secondary"`
	Success      string `json:"success"`
	Warning      string `json:"warning"`
	Danger       string `json:"danger"`
	Muted        string `json:"muted"`
	Dim          string `json:"dim"`
	White        string `json:"white"`
}

// Built-in Themes
var (
	TerminalTheme = Theme{
		Name:         "terminal",
		Description:  "Native terminal colors with transparent background",
		Transparent:  true,
		Bg:           "",
		Surface:      "",
		CardBg:       "",
		Border:       "8", // ANSI Bright Black / Gray
		BorderActive: "5", // ANSI Magenta
		Primary:      "5", // ANSI Magenta
		Secondary:    "6", // ANSI Cyan
		Success:      "2", // ANSI Green
		Warning:      "3", // ANSI Yellow
		Danger:       "1", // ANSI Red
		Muted:        "8", // ANSI Bright Black
		Dim:          "8", // ANSI Bright Black
		White:        "7", // ANSI White / Default Text
	}

	CatppuccinNoirTheme = Theme{
		Name:         "catppuccin-noir",
		Description:  "Catppuccin Noir OLED pure black palette",
		Transparent:  false,
		Bg:           "#000000",
		Surface:      "#09090d",
		CardBg:       "#111118",
		Border:       "#2a2b38",
		BorderActive: "#cba6f7",
		Primary:      "#cba6f7",
		Secondary:    "#74c7ec",
		Success:      "#a6e3a1",
		Warning:      "#f9e2af",
		Danger:       "#f38ba8",
		Muted:        "#6c7086",
		Dim:          "#45475a",
		White:        "#cdd6f4",
	}

	DraculaTheme = Theme{
		Name:         "dracula",
		Description:  "Classic dark vampire theme",
		Transparent:  false,
		Bg:           "#282a36",
		Surface:      "#21222c",
		CardBg:       "#343746",
		Border:       "#6272a4",
		BorderActive: "#bd93f9",
		Primary:      "#bd93f9",
		Secondary:    "#8be9fd",
		Success:      "#50fa7b",
		Warning:      "#f1fa8c",
		Danger:       "#ff5555",
		Muted:        "#6272a4",
		Dim:          "#44475a",
		White:        "#f8f8f2",
	}

	TokyoNightTheme = Theme{
		Name:         "tokyo-night",
		Description:  "Clean dark Tokyo Night city lights",
		Transparent:  false,
		Bg:           "#1a1b26",
		Surface:      "#16161e",
		CardBg:       "#24283b",
		Border:       "#3b4261",
		BorderActive: "#7aa2f7",
		Primary:      "#bb9af7",
		Secondary:    "#7dcfff",
		Success:      "#9ece6a",
		Warning:      "#e0af68",
		Danger:       "#f7768e",
		Muted:        "#565f89",
		Dim:          "#414868",
		White:        "#c0caf5",
	}
)

var BuiltinThemes = map[string]Theme{
	"terminal":        TerminalTheme,
	"catppuccin-noir": CatppuccinNoirTheme,
	"dracula":         DraculaTheme,
	"tokyo-night":     TokyoNightTheme,
}

// Color converts a theme color definition into a lipgloss TerminalColor.
func Color(val string) lipgloss.TerminalColor {
	if val == "" {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(val)
}

// UserThemeDir returns the default user theme directory ~/.config/trinity/themes.
func UserThemeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "trinity", "themes")
	}
	return filepath.Join(home, ".config", "trinity", "themes")
}

// ThemeSearchDirs returns all directories checked for themes.
func ThemeSearchDirs(rootDir string) []string {
	var dirs []string
	if rootDir != "" {
		dirs = append(dirs,
			filepath.Join(rootDir, ".trinity", "themes"),
			filepath.Join(rootDir, ".dev-tui", "themes"),
		)
	}
	home, err := os.UserHomeDir()
	if err == nil {
		dirs = append(dirs,
			filepath.Join(home, ".config", "trinity", "themes"),
			filepath.Join(home, ".config", "dev-tui", "themes"),
		)
	}
	return dirs
}

// LoadTheme loads a theme by name: checks built-in themes first, then searches theme directories.
func LoadTheme(name string, rootDir string) (*Theme, error) {
	if name == "" || strings.EqualFold(name, "default") {
		name = "terminal"
	}

	normName := strings.ToLower(strings.TrimSpace(name))
	if t, ok := BuiltinThemes[normName]; ok {
		cp := t
		return &cp, nil
	}

	// Check on disk
	dirs := ThemeSearchDirs(rootDir)
	for _, dir := range dirs {
		candidates := []string{
			filepath.Join(dir, normName+".json"),
			filepath.Join(dir, normName+".yaml"),
			filepath.Join(dir, normName+".yml"),
		}
		for _, path := range candidates {
			data, err := os.ReadFile(path)
			if err == nil {
				var th Theme
				if err := json.Unmarshal(data, &th); err == nil && th.Name != "" {
					return &th, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("theme %q not found", name)
}

// ListThemes returns all available themes (built-in and installed).
func ListThemes(rootDir string) []Theme {
	themesMap := make(map[string]Theme)
	for k, v := range BuiltinThemes {
		themesMap[k] = v
	}

	for _, dir := range ThemeSearchDirs(rootDir) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var th Theme
			if err := json.Unmarshal(data, &th); err == nil && th.Name != "" {
				themesMap[strings.ToLower(th.Name)] = th
			}
		}
	}

	result := make([]Theme, 0, len(themesMap))
	for _, th := range themesMap {
		result = append(result, th)
	}
	sort.Slice(result, func(i, j int) bool {
		// keep terminal first
		if result[i].Name == "terminal" {
			return true
		}
		if result[j].Name == "terminal" {
			return false
		}
		return result[i].Name < result[j].Name
	})

	return result
}

// InstallTheme installs a theme from a file path or URL into ~/.config/trinity/themes.
func InstallTheme(src string) (*Theme, string, error) {
	if src == "" {
		return nil, "", errors.New("source path or URL is required")
	}

	var data []byte
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(src)
		if err != nil {
			return nil, "", fmt.Errorf("failed to fetch theme URL: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("HTTP request failed with status: %s", resp.Status)
		}
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read response: %w", err)
		}
	} else {
		var err error
		data, err = os.ReadFile(src)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read theme file: %w", err)
		}
	}

	var th Theme
	if err := json.Unmarshal(data, &th); err != nil {
		return nil, "", fmt.Errorf("invalid theme JSON: %w", err)
	}
	if th.Name == "" {
		return nil, "", errors.New("theme JSON must include a valid 'name' field")
	}

	targetDir := UserThemeDir()
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create theme directory: %w", err)
	}

	fileName := strings.ToLower(strings.TrimSpace(th.Name)) + ".json"
	targetPath := filepath.Join(targetDir, fileName)

	// Format nicely
	pretty, err := json.MarshalIndent(th, "", "  ")
	if err != nil {
		pretty = data
	}

	if err := os.WriteFile(targetPath, pretty, 0644); err != nil {
		return nil, "", fmt.Errorf("failed to write theme file: %w", err)
	}

	return &th, targetPath, nil
}
