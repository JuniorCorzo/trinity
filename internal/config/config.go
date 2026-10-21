package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

	usedDirs := make(map[string]bool)

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

	// 2. Detect Backend (Spring Boot Java or Node.js)
	backendDirs := []string{"server", "backend", "api", "service", "backend-java", "spring-boot", "app"}
	backendFound := false

	for _, bdir := range backendDirs {
		bpath := filepath.Join(rootDir, bdir)
		fi, err := os.Stat(bpath)
		if err != nil || !fi.IsDir() {
			continue
		}

		// Check Spring Boot in backend dir
		if sbCfg, ok := detectSpringBoot(bpath, rootDir); ok {
			sbCfg.Name = "Backend"
			cfg.Services = append(cfg.Services, *sbCfg)
			backendFound = true
			usedDirs[bdir] = true
			break
		}

		// Check Node backend
		pkgPath := filepath.Join(bpath, "package.json")
		if _, err := os.Stat(pkgPath); err == nil {
			port := detectPort(bpath, 3001)
			runner := detectRunner(rootDir)
			cfg.Services = append(cfg.Services, ServiceConfig{
				Name:      "Backend",
				Command:   fmt.Sprintf("%s run dev", runner),
				Dir:       bdir,
				Port:      port,
				Type:      TypeProcess,
				AutoStart: true,
			})
			backendFound = true
			usedDirs[bdir] = true
			break
		}
	}

	// If no backend folder matched, check if root itself is a Spring Boot project
	if !backendFound {
		if sbCfg, ok := detectSpringBoot(rootDir, rootDir); ok {
			sbCfg.Name = "Backend"
			cfg.Services = append(cfg.Services, *sbCfg)
			backendFound = true
			usedDirs["."] = true
		}
	}

	// 3. Detect Frontend
	frontendDirs := []string{"web", "frontend", "client", "ui", "app", "frontend-web"}
	for _, fdir := range frontendDirs {
		if usedDirs[fdir] {
			continue
		}
		fpath := filepath.Join(rootDir, fdir)
		pkgPath := filepath.Join(fpath, "package.json")
		if _, err := os.Stat(pkgPath); err == nil {
			port := detectPort(fpath, 5173)
			runner := detectRunner(rootDir)
			cfg.Services = append(cfg.Services, ServiceConfig{
				Name:      "Frontend",
				Command:   fmt.Sprintf("%s run dev", runner),
				Dir:       fdir,
				Port:      port,
				Type:      TypeProcess,
				AutoStart: true,
			})
			usedDirs[fdir] = true
			break
		}
	}

	// Fallback if neither backend nor frontend directories matched
	if len(cfg.Services) == 0 || (len(cfg.Services) == 1 && cfg.Services[0].Type == TypeCompose) {
		rootPkg := filepath.Join(rootDir, "package.json")
		if _, err := os.Stat(rootPkg); err == nil {
			runner := detectRunner(rootDir)
			cfg.Services = append(cfg.Services, ServiceConfig{
				Name:      "Dev",
				Command:   fmt.Sprintf("%s run dev", runner),
				Dir:       ".",
				Port:      detectPort(rootDir, 3000),
				Type:      TypeProcess,
				AutoStart: true,
			})
		}
	}

	return cfg, nil
}

// detectSpringBoot checks whether dir contains a Maven or Gradle Spring Boot project
func detectSpringBoot(dir string, rootDir string) (*ServiceConfig, bool) {
	relDir, err := filepath.Rel(rootDir, dir)
	if err != nil || relDir == "" {
		relDir = "."
	}

	// 1. Check Maven (pom.xml)
	pomPath := filepath.Join(dir, "pom.xml")
	if data, err := os.ReadFile(pomPath); err == nil {
		content := string(data)
		if strings.Contains(content, "spring-boot") || strings.Contains(content, "org.springframework") {
			cmd := "mvn spring-boot:run"
			if _, err := os.Stat(filepath.Join(dir, "mvnw")); err == nil {
				cmd = "./mvnw spring-boot:run"
			} else if relPath, err := filepath.Rel(dir, filepath.Join(rootDir, "mvnw")); err == nil {
				if _, statErr := os.Stat(filepath.Join(rootDir, "mvnw")); statErr == nil {
					cmd = fmt.Sprintf("%s spring-boot:run", relPath)
				}
			}

			port := detectSpringBootPort(dir)
			return &ServiceConfig{
				Name:      "Backend",
				Command:   cmd,
				Dir:       relDir,
				Port:      port,
				Type:      TypeProcess,
				AutoStart: true,
			}, true
		}
	}

	// 2. Check Gradle (build.gradle or build.gradle.kts)
	gradleFiles := []string{"build.gradle", "build.gradle.kts"}
	for _, gf := range gradleFiles {
		gPath := filepath.Join(dir, gf)
		if data, err := os.ReadFile(gPath); err == nil {
			content := string(data)
			if strings.Contains(content, "spring-boot") || strings.Contains(content, "org.springframework.boot") {
				cmd := "gradle bootRun"
				if _, err := os.Stat(filepath.Join(dir, "gradlew")); err == nil {
					cmd = "./gradlew bootRun"
				} else if relPath, err := filepath.Rel(dir, filepath.Join(rootDir, "gradlew")); err == nil {
					if _, statErr := os.Stat(filepath.Join(rootDir, "gradlew")); statErr == nil {
						cmd = fmt.Sprintf("%s bootRun", relPath)
					}
				}

				port := detectSpringBootPort(dir)
				return &ServiceConfig{
					Name:      "Backend",
					Command:   cmd,
					Dir:       relDir,
					Port:      port,
					Type:      TypeProcess,
					AutoStart: true,
				}, true
			}
		}
	}

	return nil, false
}

// detectSpringBootPort checks application configuration files for server.port or PORT env
func detectSpringBootPort(dir string) int {
	candidates := []string{
		"src/main/resources/application-local.properties",
		"src/main/resources/application-local.yml",
		"src/main/resources/application-local.yaml",
		"src/main/resources/application-dev.properties",
		"src/main/resources/application-dev.yml",
		"src/main/resources/application-dev.yaml",
		"src/main/resources/application.properties",
		"src/main/resources/application.yml",
		"src/main/resources/application.yaml",
		"config/application.properties",
		"config/application.yml",
		"config/application.yaml",
		"application.properties",
		"application.yml",
		"application.yaml",
	}

	for _, cand := range candidates {
		data, err := os.ReadFile(filepath.Join(dir, cand))
		if err != nil {
			continue
		}
		if strings.HasSuffix(cand, ".properties") {
			if port := parsePropertiesPort(data); port > 0 {
				return port
			}
		} else {
			if port := parseYamlPort(data); port > 0 {
				return port
			}
		}
	}

	if envPort := detectEnvPort(dir); envPort > 0 {
		return envPort
	}

	return 8080
}

func parsePropertiesPort(data []byte) int {
	lines := strings.Split(string(data), "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "server.port") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) < 2 {
				parts = strings.SplitN(line, ":", 2)
			}
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				if p := extractPortNumber(val); p > 0 {
					return p
				}
			}
		}
	}
	return 0
}

func parseYamlPort(data []byte) int {
	lines := strings.Split(string(data), "\n")
	inServerBlock := false
	serverIndent := 0

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Direct inline key: server.port: 8080
		if strings.HasPrefix(trimmed, "server.port:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "server.port:"))
			if p := extractPortNumber(val); p > 0 {
				return p
			}
		}

		indent := len(rawLine) - len(strings.TrimLeft(rawLine, " "))
		if strings.HasPrefix(trimmed, "server:") {
			inServerBlock = true
			serverIndent = indent
			continue
		}

		if inServerBlock {
			if indent <= serverIndent && !strings.HasPrefix(trimmed, "-") {
				inServerBlock = false
				continue
			}
			if strings.HasPrefix(trimmed, "port:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "port:"))
				if p := extractPortNumber(val); p > 0 {
					return p
				}
			}
		}
	}
	return 0
}

var placeholderRegex = regexp.MustCompile(`\$\{[^:]+:([0-9]+)\}`)
var digitsRegex = regexp.MustCompile(`[0-9]+`)

func extractPortNumber(val string) int {
	val = strings.Trim(val, `"' `)
	if m := placeholderRegex.FindStringSubmatch(val); len(m) > 1 {
		if p, err := strconv.Atoi(m[1]); err == nil {
			return p
		}
	}
	if m := digitsRegex.FindString(val); m != "" {
		if p, err := strconv.Atoi(m); err == nil {
			return p
		}
	}
	return 0
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

func detectEnvPort(dir string) int {
	envFiles := []string{".env", ".env.local", ".env.development"}
	portRegex := regexp.MustCompile(`(?m)^(?:SERVER_PORT|PORT)\s*=\s*([0-9]+)`)
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
	return 0
}

func detectPort(dir string, defaultPort int) int {
	if p := detectEnvPort(dir); p > 0 {
		return p
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
