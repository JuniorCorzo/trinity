package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// ServiceType indicates whether a service is a local process or docker compose
type ServiceType string

const (
	TypeProcess ServiceType = "process"
	TypeCompose ServiceType = "compose"
)

// ServiceConfig defines configuration for one runnable service
type ServiceConfig struct {
	Name        string      `json:"name"`
	Command     string      `json:"command"`
	Dir         string      `json:"dir"`
	Port        int         `json:"port"`
	Type        ServiceType `json:"type"`
	AutoStart   bool        `json:"autoStart"`
	ComposeFile string      `json:"composeFile,omitempty"`
}

// Config defines the top-level TUI configuration
type Config struct {
	Title    string          `json:"title"`
	Services []ServiceConfig `json:"services"`
}

var ConfigFileCandidates = []string{
	".dev-tui.json",
	"dev-tui.json",
	"tui.config.json",
}

// LoadOrDetect loads explicit config if present, or auto-detects from the target workspace
func LoadOrDetect(rootDir string, explicitConfigPath string) (*Config, error) {
	if explicitConfigPath != "" {
		path := explicitConfigPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(rootDir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading specified config %s: %w", path, err)
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		if cfg.Title == "" {
			cfg.Title = filepath.Base(rootDir)
		}
		return &cfg, nil
	}

	for _, cand := range ConfigFileCandidates {
		p := filepath.Join(rootDir, cand)
		if data, err := os.ReadFile(p); err == nil {
			var cfg Config
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("parsing %s: %w", cand, err)
			}
			if cfg.Title == "" {
				cfg.Title = filepath.Base(rootDir)
			}
			return &cfg, nil
		}
	}

	return AutoDetect(rootDir)
}

// AutoDetect examines files in rootDir to discover backend, frontend, and docker compose
func AutoDetect(rootDir string) (*Config, error) {
	cfg := &Config{
		Title:    filepath.Base(rootDir),
		Services: make([]ServiceConfig, 0),
	}

	// 1. Detect Docker Compose
	composeFiles := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yaml", "compose.yml"}
	var foundCompose string
	for _, cf := range composeFiles {
		if _, err := os.Stat(filepath.Join(rootDir, cf)); err == nil {
			foundCompose = cf
			break
		}
	}

	if foundCompose != "" {
		port := extractComposePort(filepath.Join(rootDir, foundCompose))
		cfg.Services = append(cfg.Services, ServiceConfig{
			Name:        "Docker Compose",
			Command:     fmt.Sprintf("docker compose -f %s logs -f --tail=100", foundCompose),
			Dir:         rootDir,
			Port:        port,
			Type:        TypeCompose,
			AutoStart:   false,
			ComposeFile: foundCompose,
		})
	}

	// 2. Detect Backend
	backendDirs := []string{"server", "backend", "api"}
	for _, bdir := range backendDirs {
		pkgPath := filepath.Join(rootDir, bdir, "package.json")
		if _, err := os.Stat(pkgPath); err == nil {
			port := detectPort(filepath.Join(rootDir, bdir), 3001)
			runner := detectRunner(rootDir)
			cfg.Services = append(cfg.Services, ServiceConfig{
				Name:      "Backend",
				Command:   fmt.Sprintf("%s run dev", runner),
				Dir:       bdir,
				Port:      port,
				Type:      TypeProcess,
				AutoStart: true,
			})
			break
		}
	}

	// 3. Detect Frontend
	frontendDirs := []string{"web", "frontend", "client", "app", "ui"}
	for _, fdir := range frontendDirs {
		pkgPath := filepath.Join(rootDir, fdir, "package.json")
		if _, err := os.Stat(pkgPath); err == nil {
			port := detectPort(filepath.Join(rootDir, fdir), 5173)
			runner := detectRunner(rootDir)
			cfg.Services = append(cfg.Services, ServiceConfig{
				Name:      "Frontend",
				Command:   fmt.Sprintf("%s run dev", runner),
				Dir:       fdir,
				Port:      port,
				Type:      TypeProcess,
				AutoStart: true,
			})
			break
		}
	}

	// Fallback if neither backend nor frontend directories matched
	if len(cfg.Services) == 0 {
		rootPkg := filepath.Join(rootDir, "package.json")
		if _, err := os.Stat(rootPkg); err == nil {
			runner := detectRunner(rootDir)
			cfg.Services = append(cfg.Services, ServiceConfig{
				Name:      "Dev",
				Command:   fmt.Sprintf("%s run dev", runner),
				Dir:       ".",
				Port:      3000,
				Type:      TypeProcess,
				AutoStart: true,
			})
		}
	}

	return cfg, nil
}

func detectRunner(rootDir string) string {
	if _, err := os.Stat(filepath.Join(rootDir, "bun.lock")); err == nil {
		return "bun"
	}
	if _, err := os.Stat(filepath.Join(rootDir, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}
	if _, err := os.Stat(filepath.Join(rootDir, "yarn.lock")); err == nil {
		return "yarn"
	}
	return "npm"
}

func detectPort(dir string, defaultPort int) int {
	envFiles := []string{".env", ".env.local", ".env.development"}
	portRegex := regexp.MustCompile(`(?m)^PORT\s*=\s*([0-9]+)`)
	for _, env := range envFiles {
		data, err := os.ReadFile(filepath.Join(dir, env))
		if err == nil {
			matches := portRegex.FindSubmatch(data)
			if len(matches) > 1 {
				if p, err := strconv.Atoi(string(matches[1])); err == nil {
					return p
				}
			}
		}
	}
	return defaultPort
}

func extractComposePort(composePath string) int {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return 0
	}
	portRegex := regexp.MustCompile(`["']?([0-9]+):([0-9]+)["']?`)
	matches := portRegex.FindSubmatch(data)
	if len(matches) > 1 {
		if p, err := strconv.Atoi(string(matches[1])); err == nil {
			return p
		}
	}
	return 0
}
