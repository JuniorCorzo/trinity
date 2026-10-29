package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrConfigNotFound  = errors.New("configuration file not found")
	ErrNoServicesFound = errors.New("no services found or detected")
)

// ServiceType indicates whether a service is a local process or docker compose
type ServiceType string

const (
	TypeProcess ServiceType = "process"
	TypeCompose ServiceType = "compose"
)

// ServiceConfig defines configuration for one runnable service
type ServiceConfig struct {
	Name           string      `json:"name"`
	Command        string      `json:"command"`
	Dir            string      `json:"dir"`
	Port           int         `json:"port"`
	Type           ServiceType `json:"type"`
	AutoStart      bool        `json:"autoStart"`
	ComposeFile    string      `json:"composeFile,omitempty"`
	ComposeService string      `json:"composeService,omitempty"`
}

// Config defines the top-level TUI configuration
type Config struct {
	Title    string          `json:"title"`
	Theme    string          `json:"theme,omitempty"`
	Services []ServiceConfig `json:"services"`
}

var ConfigFileCandidates = []string{
	".trinity.json",
	"trinity.json",
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
			return nil, fmt.Errorf("%w: reading specified config %s: %v", ErrConfigNotFound, path, err)
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

// InitConfigFile generates a starter .dev-tui.json configuration file in rootDir.
// If force is false and a configuration file already exists, it returns an error.
func InitConfigFile(rootDir string, force bool) (string, error) {
	targetPath := filepath.Join(rootDir, ".dev-tui.json")
	if !force {
		for _, cand := range ConfigFileCandidates {
			p := filepath.Join(rootDir, cand)
			if _, err := os.Stat(p); err == nil {
				return "", fmt.Errorf("configuration file %s already exists (use --force to overwrite)", cand)
			}
		}
	}

	detected, err := AutoDetect(rootDir)
	var cfg *Config
	if err == nil && len(detected.Services) > 0 {
		cfg = detected
	} else {
		cfg = &Config{
			Title: filepath.Base(rootDir),
			Services: []ServiceConfig{
				{
					Name:      "Backend",
					Dir:       "server",
					Command:   "bun run dev",
					Port:      3001,
					Type:      TypeProcess,
					AutoStart: true,
				},
				{
					Name:      "Frontend",
					Dir:       "web",
					Command:   "bun run dev",
					Port:      5173,
					Type:      TypeProcess,
					AutoStart: true,
				},
			},
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling configuration: %w", err)
	}

	data = append(data, '\n')
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", targetPath, err)
	}

	return targetPath, nil
}

var (
	ViteConfigFiles = []string{
		"vite.config.ts", "vite.config.js", "vite.config.mjs", "vite.config.cjs",
	}
	NextConfigFiles = []string{
		"next.config.js", "next.config.mjs", "next.config.ts",
	}
	NuxtConfigFiles = []string{
		"nuxt.config.ts", "nuxt.config.js",
	}
	AstroConfigFiles = []string{
		"astro.config.mjs", "astro.config.ts", "astro.config.js",
	}
	SvelteConfigFiles = []string{
		"svelte.config.js", "svelte.config.ts",
	}
	AngularConfigFiles = []string{
		"angular.json",
	}
	NestConfigFiles = []string{
		"nest-cli.json",
	}
	GoConfigFiles = []string{
		"go.mod",
	}
)

func hasAnyFile(dir string, filenames []string) (string, bool) {
	for _, f := range filenames {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return f, true
		}
	}
	return "", false
}

func extractVitePort(dir string) int {
	portRegex := regexp.MustCompile(`port\s*:\s*([0-9]+)`)
	for _, cand := range ViteConfigFiles {
		data, err := os.ReadFile(filepath.Join(dir, cand))
		if err != nil {
			continue
		}
		matches := portRegex.FindSubmatch(data)
		if len(matches) > 1 {
			if p, err := strconv.Atoi(string(matches[1])); err == nil && p > 0 {
				return p
			}
		}
	}
	return 0
}

func isNodeServer(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	content := string(data)
	serverDeps := []string{
		"\"express\"", "\"fastify\"", "\"@nestjs/core\"", "\"koa\"",
		"\"hono\"", "\"@scalar/express-api-reference\"",
		"\"drizzle-orm\"", "\"prisma\"", "\"typeorm\"",
		"\"mongoose\"", "\"pg\"", "\"mysql2\"", "\"mongodb\"",
	}
	for _, dep := range serverDeps {
		if strings.Contains(content, dep) {
			return true
		}
	}
	return false
}

func detectPythonBackend(dir string, relDir string) (*ServiceConfig, bool) {
	pyproject := filepath.Join(dir, "pyproject.toml")
	reqs := filepath.Join(dir, "requirements.txt")
	content := ""
	if data, err := os.ReadFile(pyproject); err == nil {
		content += string(data) + "\n"
	}
	if data, err := os.ReadFile(reqs); err == nil {
		content += string(data) + "\n"
	}
	if content == "" {
		return nil, false
	}

	lower := strings.ToLower(content)
	cmd := ""
	port := 8000
	if strings.Contains(lower, "fastapi") || strings.Contains(lower, "uvicorn") {
		cmd = "uvicorn main:app --reload"
		port = detectPort(dir, 8000)
	} else if strings.Contains(lower, "flask") {
		cmd = "flask run"
		port = detectPort(dir, 5000)
	} else if strings.Contains(lower, "django") {
		cmd = "python manage.py runserver"
		port = detectPort(dir, 8000)
	}

	if cmd == "" {
		return nil, false
	}

	return &ServiceConfig{
		Name:      "Backend",
		Command:   cmd,
		Dir:       relDir,
		Port:      port,
		Type:      TypeProcess,
		AutoStart: true,
	}, true
}

func collectCandidateDirs(rootDir string) []string {
	var dirs []string
	ignored := map[string]bool{
		"node_modules": true, "scripts": true, ".git": true, ".agents": true,
		".gemini": true, ".codegraph": true, ".engram": true, "dist": true,
		"build": true, "public": true, "static": true, "assets": true,
		".idea": true, ".vscode": true, "target": true, "vendor": true,
		"core": true,
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || ignored[name] {
			continue
		}
		dirs = append(dirs, name)

		// Also check monorepo folders: apps/, packages/, services/
		if name == "apps" || name == "packages" || name == "services" {
			subEntries, err := os.ReadDir(filepath.Join(rootDir, name))
			if err == nil {
				for _, se := range subEntries {
					if !se.IsDir() {
						continue
					}
					sname := se.Name()
					if strings.HasPrefix(sname, ".") || ignored[sname] {
						continue
					}
					dirs = append(dirs, filepath.Join(name, sname))
				}
			}
		}
	}

	return dirs
}

func inspectSubdirectory(fullPath string, relDir string, rootDir string) (*ServiceConfig, bool) {
	// 1. Vite (React, Vue, Svelte, Solid, etc. via vite.config.*)
	if _, ok := hasAnyFile(fullPath, ViteConfigFiles); ok {
		port := extractVitePort(fullPath)
		if port == 0 {
			port = detectPort(fullPath, 5173)
		}
		runner := detectRunner(fullPath, rootDir)
		cmd := detectPackageDevCommand(fullPath, runner)
		name := "Frontend"
		base := strings.ToLower(filepath.Base(relDir))
		if base != "web" && base != "frontend" && base != "client" && base != "ui" && base != "app" {
			name = strings.Title(base)
		}
		return &ServiceConfig{
			Name:      name,
			Command:   cmd,
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	// 2. Next.js (next.config.*)
	if _, ok := hasAnyFile(fullPath, NextConfigFiles); ok {
		port := detectPort(fullPath, 3000)
		runner := detectRunner(fullPath, rootDir)
		cmd := detectPackageDevCommand(fullPath, runner)
		name := "Frontend"
		base := strings.ToLower(filepath.Base(relDir))
		if base != "web" && base != "frontend" && base != "client" && base != "ui" && base != "app" {
			name = strings.Title(base)
		}
		return &ServiceConfig{
			Name:      name,
			Command:   cmd,
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	// 3. Nuxt (nuxt.config.*)
	if _, ok := hasAnyFile(fullPath, NuxtConfigFiles); ok {
		port := detectPort(fullPath, 3000)
		runner := detectRunner(fullPath, rootDir)
		cmd := detectPackageDevCommand(fullPath, runner)
		return &ServiceConfig{
			Name:      "Frontend",
			Command:   cmd,
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	// 4. Astro (astro.config.*)
	if _, ok := hasAnyFile(fullPath, AstroConfigFiles); ok {
		port := detectPort(fullPath, 4321)
		runner := detectRunner(fullPath, rootDir)
		cmd := detectPackageDevCommand(fullPath, runner)
		return &ServiceConfig{
			Name:      "Frontend",
			Command:   cmd,
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	// 5. SvelteKit (svelte.config.js)
	if _, ok := hasAnyFile(fullPath, SvelteConfigFiles); ok {
		port := detectPort(fullPath, 5173)
		runner := detectRunner(fullPath, rootDir)
		cmd := detectPackageDevCommand(fullPath, runner)
		return &ServiceConfig{
			Name:      "Frontend",
			Command:   cmd,
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	// 6. Angular (angular.json)
	if _, ok := hasAnyFile(fullPath, AngularConfigFiles); ok {
		port := detectPort(fullPath, 4200)
		runner := detectRunner(fullPath, rootDir)
		cmd := detectPackageDevCommand(fullPath, runner)
		return &ServiceConfig{
			Name:      "Frontend",
			Command:   cmd,
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	// 7. Java Spring Boot (Maven pom.xml or Gradle build.gradle)
	if sbCfg, ok := detectSpringBoot(fullPath, rootDir); ok {
		sbCfg.Name = "Backend"
		sbCfg.Dir = relDir
		return sbCfg, true
	}

	// 8. NestJS (nest-cli.json)
	if _, ok := hasAnyFile(fullPath, NestConfigFiles); ok {
		port := detectPort(fullPath, 3000)
		runner := detectRunner(fullPath, rootDir)
		cmd := detectPackageDevCommand(fullPath, runner)
		return &ServiceConfig{
			Name:      "Backend",
			Command:   cmd,
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	// 9. Go module (go.mod)
	if _, ok := hasAnyFile(fullPath, GoConfigFiles); ok {
		port := detectPort(fullPath, 8080)
		return &ServiceConfig{
			Name:      "Backend",
			Command:   "go run .",
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	// 10. Python Backend (pyproject.toml, requirements.txt)
	if pyCfg, ok := detectPythonBackend(fullPath, relDir); ok {
		return pyCfg, true
	}

	// 11. Node Server by package.json dependencies (express, fastify, hono, drizzle, prisma, etc.)
	if isNodeServer(fullPath) {
		port := detectPort(fullPath, 3001)
		runner := detectRunner(fullPath, rootDir)
		cmd := detectPackageDevCommand(fullPath, runner)
		return &ServiceConfig{
			Name:      "Backend",
			Command:   cmd,
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	// 12. General package.json in subfolders
	pkgPath := filepath.Join(fullPath, "package.json")
	if _, err := os.Stat(pkgPath); err == nil {
		runner := detectRunner(fullPath, rootDir)
		cmd := detectPackageDevCommand(fullPath, runner)
		base := strings.ToLower(filepath.Base(relDir))
		name := strings.Title(base)
		portDefault := 0
		if base == "server" || base == "backend" || base == "api" {
			name = "Backend"
			portDefault = 3001
		} else if base == "web" || base == "frontend" || base == "client" || base == "ui" {
			name = "Frontend"
			portDefault = 5173
		}
		port := detectPort(fullPath, portDefault)
		return &ServiceConfig{
			Name:      name,
			Command:   cmd,
			Dir:       relDir,
			Port:      port,
			Type:      TypeProcess,
			AutoStart: true,
		}, true
	}

	return nil, false
}

// AutoDetect examines files in rootDir to discover backend and frontend native processes.
// It prioritizes framework configuration files (pom.xml, vite.config, next.config, etc.)
// for reactive development (hot reload, watch mode, instant restarts) and strictly ignores root package.json.
// Docker compose services are added if no native services exist, or as auxiliary infrastructure (databases, redis).
func AutoDetect(rootDir string) (*Config, error) {
	cfg := &Config{
		Title:    filepath.Base(rootDir),
		Services: make([]ServiceConfig, 0),
	}

	candidateDirs := collectCandidateDirs(rootDir)
	usedDirs := make(map[string]bool)

	var backends []ServiceConfig
	var frontends []ServiceConfig
	var others []ServiceConfig

	for _, relDir := range candidateDirs {
		fullPath := filepath.Join(rootDir, relDir)
		fi, err := os.Stat(fullPath)
		if err != nil || !fi.IsDir() {
			continue
		}

		svc, ok := inspectSubdirectory(fullPath, relDir, rootDir)
		if !ok {
			continue
		}

		usedDirs[relDir] = true
		if svc.Name == "Backend" {
			backends = append(backends, *svc)
		} else if svc.Name == "Frontend" {
			frontends = append(frontends, *svc)
		} else {
			others = append(others, *svc)
		}
	}

	// Check root for standalone Spring Boot or Go backend if no backend in subfolders
	if len(backends) == 0 {
		if sbCfg, ok := detectSpringBoot(rootDir, rootDir); ok {
			sbCfg.Name = "Backend"
			backends = append(backends, *sbCfg)
		} else if _, err := os.Stat(filepath.Join(rootDir, "go.mod")); err == nil {
			backends = append(backends, ServiceConfig{
				Name:      "Backend",
				Command:   "go run .",
				Dir:       ".",
				Port:      detectPort(rootDir, 8080),
				Type:      TypeProcess,
				AutoStart: true,
			})
		}
	}

	// If multiple backends or frontends exist, give them folder-qualified names
	if len(backends) > 1 {
		for i := range backends {
			backends[i].Name = fmt.Sprintf("Backend (%s)", filepath.Base(backends[i].Dir))
		}
	}
	if len(frontends) > 1 {
		for i := range frontends {
			frontends[i].Name = fmt.Sprintf("Frontend (%s)", filepath.Base(frontends[i].Dir))
		}
	}

	cfg.Services = append(cfg.Services, backends...)
	cfg.Services = append(cfg.Services, frontends...)
	cfg.Services = append(cfg.Services, others...)

	// Docker Compose Integration
	foundCompose := DetectComposeFile(rootDir)
	if foundCompose != "" {
		composePath := filepath.Join(rootDir, foundCompose)
		composeServices, err := ParseComposeFile(composePath)
		if err == nil && len(composeServices) > 0 {
			if len(cfg.Services) == 0 {
				// No native processes found; use all compose services
				for _, cs := range composeServices {
					cfg.Services = append(cfg.Services, ServiceConfig{
						Name:           cs.Name,
						Command:        fmt.Sprintf("docker compose -f %s logs -f --tail=100 %s", foundCompose, cs.Name),
						Dir:            rootDir,
						Port:           cs.Port,
						Type:           TypeCompose,
						AutoStart:      true,
						ComposeFile:    foundCompose,
						ComposeService: cs.Name,
					})
				}
			} else {
				// Native processes already exist (e.g., Backend, Frontend).
				// Only import infrastructure / database / cache services from compose so they don't conflict.
				for _, cs := range composeServices {
					if isInfraService(cs.Name) {
						cfg.Services = append(cfg.Services, ServiceConfig{
							Name:           cs.Name,
							Command:        fmt.Sprintf("docker compose -f %s logs -f --tail=100 %s", foundCompose, cs.Name),
							Dir:            rootDir,
							Port:           cs.Port,
							Type:           TypeCompose,
							AutoStart:      true,
							ComposeFile:    foundCompose,
							ComposeService: cs.Name,
						})
					}
				}
			}
		} else if len(cfg.Services) == 0 {
			port := extractComposePort(composePath)
			cfg.Services = append(cfg.Services, ServiceConfig{
				Name:        "Docker Compose",
				Command:     fmt.Sprintf("docker compose -f %s logs -f --tail=100", foundCompose),
				Dir:         rootDir,
				Port:        port,
				Type:        TypeCompose,
				AutoStart:   true,
				ComposeFile: foundCompose,
			})
		}
	}

	// NOTE: We deliberately DO NOT fall back to any root package.json startup script.
	// As requested, root package.json is ignored to prevent running unreactive wrap scripts.

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

func detectRunner(dirs ...string) string {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(d, "bun.lock")); err == nil {
			return "bun"
		}
		if _, err := os.Stat(filepath.Join(d, "bun.lockb")); err == nil {
			return "bun"
		}
		if _, err := os.Stat(filepath.Join(d, "pnpm-lock.yaml")); err == nil {
			return "pnpm"
		}
		if _, err := os.Stat(filepath.Join(d, "yarn.lock")); err == nil {
			return "yarn"
		}
	}
	return "npm"
}

func detectPackageDevCommand(dir string, runner string) string {
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Scripts != nil {
			if _, ok := pkg.Scripts["dev"]; ok {
				return fmt.Sprintf("%s run dev", runner)
			}
			if _, ok := pkg.Scripts["start"]; ok {
				return fmt.Sprintf("%s start", runner)
			}
		}
	}
	return fmt.Sprintf("%s run dev", runner)
}

func detectEnvPort(dir string) int {
	dirs := []string{dir}
	parent := filepath.Dir(dir)
	if parent != dir && parent != "." && parent != "/" {
		dirs = append(dirs, parent)
	}

	envFiles := []string{".env", ".env.local", ".env.development", ".env.dev"}
	portRegex := regexp.MustCompile(`(?m)^(?:SERVER_PORT|PORT|VITE_PORT)\s*=\s*([0-9]+)`)
	for _, d := range dirs {
		for _, env := range envFiles {
			data, err := os.ReadFile(filepath.Join(d, env))
			if err == nil {
				matches := portRegex.FindSubmatch(data)
				if len(matches) > 1 {
					if p, err := strconv.Atoi(string(matches[1])); err == nil {
						return p
					}
				}
			}
		}
	}
	return 0
}

func isInfraService(name string) bool {
	lower := strings.ToLower(name)
	infraKeywords := []string{
		"db", "database", "postgres", "psql", "mysql", "mariadb",
		"redis", "mongo", "mongodb", "rabbitmq", "rabbit",
		"kafka", "zookeeper", "localstack", "mailhog", "mailpit",
		"memcached", "elasticsearch", "elastic", "minio",
	}
	for _, kw := range infraKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func detectPort(dir string, defaultPort int) int {
	if p := detectEnvPort(dir); p > 0 {
		return p
	}
	return defaultPort
}

func extractComposePort(composePath string) int {
	services, err := ParseComposeFile(composePath)
	if err == nil && len(services) > 0 {
		for _, s := range services {
			if s.Port > 0 {
				return s.Port
			}
		}
	}

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

// ComposeDevPrefixCandidates are common compose file names with dev/development prefix
var ComposeDevPrefixCandidates = []string{
	"dev.docker-compose.yml",
	"dev.docker-compose.yaml",
	"dev-docker-compose.yml",
	"dev-docker-compose.yaml",
	"dev_docker-compose.yml",
	"dev_docker-compose.yaml",
	"dev.compose.yml",
	"dev.compose.yaml",
	"dev-compose.yml",
	"dev-compose.yaml",
	"dev_compose.yml",
	"dev_compose.yaml",
	"development.docker-compose.yml",
	"development.docker-compose.yaml",
	"development-docker-compose.yml",
	"development-docker-compose.yaml",
	"development.compose.yml",
	"development.compose.yaml",
	"development-compose.yml",
	"development-compose.yaml",
}

// ComposeDevInfixCandidates are common compose file names with dev/development infix/suffix
var ComposeDevInfixCandidates = []string{
	"docker-compose.dev.yml",
	"docker-compose.dev.yaml",
	"docker-compose-dev.yml",
	"docker-compose-dev.yaml",
	"docker-compose_dev.yml",
	"docker-compose_dev.yaml",
	"compose.dev.yml",
	"compose.dev.yaml",
	"compose-dev.yml",
	"compose-dev.yaml",
	"compose_dev.yml",
	"compose_dev.yaml",
	"docker-compose.development.yml",
	"docker-compose.development.yaml",
	"docker-compose-development.yml",
	"docker-compose-development.yaml",
	"compose.development.yml",
	"compose.development.yaml",
	"compose-development.yml",
	"compose-development.yaml",
}

var ComposeStandardCandidates = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yaml",
	"compose.yml",
}

// ComposeSearchDirs are common subdirectories where docker compose files live
var ComposeSearchDirs = []string{
	".",
	"docker",
	".docker",
	"compose",
	".compose",
	"deploy",
	"deployment",
	"deployments",
	"infra",
	"infrastructure",
}

// DetectComposeFile discovers the appropriate docker compose file in rootDir or common subdirectories.
// In development tooling, compose files with dev/development prefixes/infixes are prioritized.
func DetectComposeFile(rootDir string) string {
	// 1. Check known dev prefix candidates across search dirs
	for _, dir := range ComposeSearchDirs {
		for _, cand := range ComposeDevPrefixCandidates {
			rel := filepath.Join(dir, cand)
			if _, err := os.Stat(filepath.Join(rootDir, rel)); err == nil {
				return rel
			}
		}
	}

	// 2. Check known dev infix/suffix candidates across search dirs
	for _, dir := range ComposeSearchDirs {
		for _, cand := range ComposeDevInfixCandidates {
			rel := filepath.Join(dir, cand)
			if _, err := os.Stat(filepath.Join(rootDir, rel)); err == nil {
				return rel
			}
		}
	}

	// 3. Scan directory entries to catch custom named dev files across search dirs
	for _, dir := range ComposeSearchDirs {
		targetDir := filepath.Join(rootDir, dir)
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			continue
		}
		var devPrefixCustom []string
		var devCustom []string

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			lower := strings.ToLower(name)
			if !(strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")) {
				continue
			}
			if !strings.Contains(lower, "compose") {
				continue
			}

			isDev := strings.Contains(lower, "dev") || strings.Contains(lower, "development")
			hasDevPrefix := strings.HasPrefix(lower, "dev") || strings.HasPrefix(lower, "development")

			if isDev && hasDevPrefix {
				devPrefixCustom = append(devPrefixCustom, name)
			} else if isDev {
				devCustom = append(devCustom, name)
			}
		}

		if len(devPrefixCustom) > 0 {
			sort.Strings(devPrefixCustom)
			return filepath.Join(dir, devPrefixCustom[0])
		}
		if len(devCustom) > 0 {
			sort.Strings(devCustom)
			return filepath.Join(dir, devCustom[0])
		}
	}

	// 4. Fall back to standard compose file names across search dirs
	for _, dir := range ComposeSearchDirs {
		for _, cand := range ComposeStandardCandidates {
			rel := filepath.Join(dir, cand)
			if _, err := os.Stat(filepath.Join(rootDir, rel)); err == nil {
				return rel
			}
		}
	}

	// 5. Any generic compose file found during scan across search dirs
	for _, dir := range ComposeSearchDirs {
		targetDir := filepath.Join(rootDir, dir)
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			lower := strings.ToLower(name)
			if (strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")) && strings.Contains(lower, "compose") {
				return filepath.Join(dir, name)
			}
		}
	}

	return ""
}

// ComposeServiceInfo represents a single detected service inside a docker compose file
type ComposeServiceInfo struct {
	Name string
	Port int
}

// ParseComposeFile parses services and published host ports from a docker compose file
func ParseComposeFile(composeFilePath string) ([]ComposeServiceInfo, error) {
	data, err := os.ReadFile(composeFilePath)
	if err != nil {
		return nil, err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	var docNode *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		docNode = root.Content[0]
	} else if root.Kind == yaml.MappingNode {
		docNode = &root
	}

	if docNode == nil || docNode.Kind != yaml.MappingNode {
		return nil, nil
	}

	var servicesNode *yaml.Node
	for i := 0; i < len(docNode.Content)-1; i += 2 {
		key := docNode.Content[i]
		if key.Value == "services" {
			servicesNode = docNode.Content[i+1]
			break
		}
	}

	if servicesNode == nil || servicesNode.Kind != yaml.MappingNode {
		return nil, nil
	}

	var services []ComposeServiceInfo
	for i := 0; i < len(servicesNode.Content)-1; i += 2 {
		svcNameNode := servicesNode.Content[i]
		svcDefNode := servicesNode.Content[i+1]
		svcName := svcNameNode.Value

		port := 0
		if svcDefNode.Kind == yaml.MappingNode {
			for j := 0; j < len(svcDefNode.Content)-1; j += 2 {
				k := svcDefNode.Content[j]
				if k.Value == "ports" {
					portsNode := svcDefNode.Content[j+1]
					if portsNode.Kind == yaml.SequenceNode {
						for _, pItem := range portsNode.Content {
							if pItem.Kind == yaml.ScalarNode {
								if parsed := extractHostPort(pItem.Value); parsed > 0 {
									port = parsed
									break
								}
							} else if pItem.Kind == yaml.MappingNode {
								for m := 0; m < len(pItem.Content)-1; m += 2 {
									if pItem.Content[m].Value == "published" {
										val := pItem.Content[m+1].Value
										if p, err := strconv.Atoi(val); err == nil && p > 0 {
											port = p
											break
										}
									}
								}
								if port > 0 {
									break
								}
							}
						}
					}
					break
				}
			}
		}

		services = append(services, ComposeServiceInfo{
			Name: svcName,
			Port: port,
		})
	}

	return services, nil
}

func extractHostPort(raw string) int {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	if raw == "" {
		return 0
	}
	if idx := strings.Index(raw, "/"); idx != -1 {
		raw = raw[:idx]
	}
	parts := strings.Split(raw, ":")
	var hostPortStr string
	switch len(parts) {
	case 1:
		return 0
	case 2:
		hostPortStr = parts[0]
	default:
		hostPortStr = parts[len(parts)-2]
	}

	if idx := strings.Index(hostPortStr, "-"); idx != -1 {
		hostPortStr = hostPortStr[:idx]
	}

	p, err := strconv.Atoi(strings.TrimSpace(hostPortStr))
	if err == nil && p > 0 && p <= 65535 {
		return p
	}
	return 0
}
