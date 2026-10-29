package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutoDetect(t *testing.T) {
	tests := []struct {
		name         string
		setupFiles   map[string]string
		wantServices []string
		wantPorts    map[string]int
		wantCommands map[string]string
	}{
		{
			name: "prioritizes native processes in server and web subfolders and includes aux db from compose",
			setupFiles: map[string]string{
				"bun.lock":            "",
				"server/package.json": `{"name": "server", "scripts": {"dev": "bun --watch src/index.ts"}}`,
				"server/.env":         "PORT=3001\n",
				"web/package.json":    `{"name": "web", "scripts": {"dev": "vite"}}`,
				"web/.env":            "PORT=5173\n",
				"docker-compose.yml":  "services:\n  app:\n    ports:\n      - \"8080:8080\"\n  db:\n    ports:\n      - \"5432:5432\"\n",
				"package.json":        `{"name": "root-wrapper", "scripts": {"dev": "./scripts/start.sh"}}`,
			},
			wantServices: []string{"Backend", "Frontend", "db"},
			wantPorts: map[string]int{
				"Backend":  3001,
				"Frontend": 5173,
				"db":       5432,
			},
			wantCommands: map[string]string{
				"Backend":  "bun run dev",
				"Frontend": "bun run dev",
				"db":       "docker compose -f docker-compose.yml logs -f --tail=100 db",
			},
		},
		{
			name: "detects java spring boot maven backend with mvnw and properties port",
			setupFiles: map[string]string{
				"backend/pom.xml":                                  `<project><groupId>com.example</groupId><artifactId>demo</artifactId><parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId><version>3.2.0</version></parent></project>`,
				"backend/mvnw":                                     `#!/bin/sh`,
				"backend/src/main/resources/application.properties": "server.port=8085\n",
				"frontend/package.json":                            `{"name": "frontend"}`,
			},
			wantServices: []string{"Backend", "Frontend"},
			wantPorts: map[string]int{
				"Backend":  8085,
				"Frontend": 5173,
			},
			wantCommands: map[string]string{
				"Backend": "./mvnw spring-boot:run",
			},
		},
		{
			name: "detects java spring boot gradle backend with gradlew and yaml port",
			setupFiles: map[string]string{
				"backend/build.gradle":                         `plugins { id 'org.springframework.boot' version '3.2.0' }`,
				"backend/gradlew":                              `#!/bin/sh`,
				"backend/src/main/resources/application.yml": "server:\n  port: 8090\n",
				"web/package.json":                             `{"name": "web"}`,
			},
			wantServices: []string{"Backend", "Frontend"},
			wantPorts: map[string]int{
				"Backend":  8090,
				"Frontend": 5173,
			},
			wantCommands: map[string]string{
				"Backend": "./gradlew bootRun",
			},
		},
		{
			name: "detects root spring boot with frontend subfolder and relative mvnw",
			setupFiles: map[string]string{
				"pom.xml":                                  `<project><dependencies><dependency><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-web</artifactId></dependency></dependencies></project>`,
				"mvnw":                                     `#!/bin/sh`,
				"src/main/resources/application.properties": "server.port=${PORT:9000}\n",
				"client/package.json":                      `{"name": "client"}`,
			},
			wantServices: []string{"Backend", "Frontend"},
			wantPorts: map[string]int{
				"Backend":  9000,
				"Frontend": 5173,
			},
			wantCommands: map[string]string{
				"Backend": "./mvnw spring-boot:run",
			},
		},
		{
			name: "detects root standalone spring boot gradle kotlin dsl default port",
			setupFiles: map[string]string{
				"build.gradle.kts": `plugins { id("org.springframework.boot") version "3.2.0" }`,
			},
			wantServices: []string{"Backend"},
			wantPorts: map[string]int{
				"Backend": 8080,
			},
			wantCommands: map[string]string{
				"Backend": "gradle bootRun",
			},
		},
		{
			name: "detects backend spring boot with root mvnw wrapper",
			setupFiles: map[string]string{
				"mvnw":            `#!/bin/sh`,
				"backend/pom.xml": `<project><parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId><version>3.2.0</version></parent></project>`,
			},
			wantServices: []string{"Backend"},
			wantPorts: map[string]int{
				"Backend": 8080,
			},
			wantCommands: map[string]string{
				"Backend": "../mvnw spring-boot:run",
			},
		},
		{
			name: "ignores root package.json startup script when no subdirectories or compose exist",
			setupFiles: map[string]string{
				"package.json": `{"name": "single-app", "scripts": {"dev": "./scripts/start.sh"}}`,
			},
			wantServices: []string{},
			wantPorts:    map[string]int{},
		},
		{
			name: "detects vite frontend with custom port and nestjs backend",
			setupFiles: map[string]string{
				"custom-ui/vite.config.ts": `export default { server: { port: 5174 } }`,
				"custom-ui/package.json":   `{"name": "custom-ui", "scripts": {"dev": "vite"}}`,
				"api/nest-cli.json":        `{}`,
				"api/package.json":         `{"name": "api", "scripts": {"dev": "nest start --watch"}}`,
			},
			wantServices: []string{"Backend", "Custom-Ui"},
			wantPorts: map[string]int{
				"Backend":   3000,
				"Custom-Ui": 5174,
			},
			wantCommands: map[string]string{
				"Backend":   "npm run dev",
				"Custom-Ui": "npm run dev",
			},
		},
		{
			name: "detects monorepo apps/web with vite and apps/api with pom.xml spring boot",
			setupFiles: map[string]string{
				"apps/web/vite.config.js":                            `export default {}`,
				"apps/web/package.json":                             `{"name": "web"}`,
				"apps/api/pom.xml":                                  `<project><groupId>com.example</groupId><artifactId>api</artifactId><parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId><version>3.2.0</version></parent></project>`,
				"apps/api/src/main/resources/application.properties": "server.port=8088\n",
			},
			wantServices: []string{"Backend", "Frontend"},
			wantPorts: map[string]int{
				"Backend":  8088,
				"Frontend": 5173,
			},
			wantCommands: map[string]string{
				"Backend":  "mvn spring-boot:run",
				"Frontend": "npm run dev",
			},
		},
		{
			name: "prioritizes dev prefix compose and extracts all compose services without script fallback",
			setupFiles: map[string]string{
				"package.json":           `{"name": "full-stack", "scripts": {"dev": "vite"}}`,
				"docker-compose.yml":     "services:\n  prod_web:\n    ports:\n      - \"80:80\"\n",
				"dev.docker-compose.yml": "services:\n  web:\n    ports:\n      - \"3000:3000\"\n  api:\n    ports:\n      - \"127.0.0.1:8080:80\"\n  db:\n    ports:\n      - \"5432:5432\"\n  redis:\n    image: redis:alpine\n",
			},
			wantServices: []string{"web", "api", "db", "redis"},
			wantPorts: map[string]int{
				"web":   3000,
				"api":   8080,
				"db":    5432,
				"redis": 0,
			},
			wantCommands: map[string]string{
				"web":   "docker compose -f dev.docker-compose.yml logs -f --tail=100 web",
				"api":   "docker compose -f dev.docker-compose.yml logs -f --tail=100 api",
				"db":    "docker compose -f dev.docker-compose.yml logs -f --tail=100 db",
				"redis": "docker compose -f dev.docker-compose.yml logs -f --tail=100 redis",
			},
		},
		{
			name: "prioritizes compose.dev.yaml with long syntax ports",
			setupFiles: map[string]string{
				"compose.yaml": "services:\n  prod:\n    ports:\n      - \"80:80\"\n",
				"compose.dev.yaml": `services:
  gateway:
    ports:
      - target: 80
        published: 9090
        protocol: tcp
  worker:
    image: worker:latest
`,
			},
			wantServices: []string{"gateway", "worker"},
			wantPorts: map[string]int{
				"gateway": 9090,
				"worker":  0,
			},
			wantCommands: map[string]string{
				"gateway": "docker compose -f compose.dev.yaml logs -f --tail=100 gateway",
				"worker":  "docker compose -f compose.dev.yaml logs -f --tail=100 worker",
			},
		},
		{
			name: "detects dev compose in subdirectories like docker/dev.docker-compose.yml",
			setupFiles: map[string]string{
				"docker-compose.yml": "services:\n  prod_db:\n    ports:\n      - \"5432:5432\"\n",
				"docker/dev.docker-compose.yml": `services:
  api:
    ports:
      - "4000:4000"
`,
			},
			wantServices: []string{"api"},
			wantPorts: map[string]int{
				"api": 4000,
			},
			wantCommands: map[string]string{
				"api": "docker compose -f docker/dev.docker-compose.yml logs -f --tail=100 api",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			for relPath, content := range tt.setupFiles {
				fullPath := filepath.Join(tmpDir, relPath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
			}

			cfg, err := AutoDetect(tmpDir)
			if err != nil {
				t.Fatalf("AutoDetect error: %v", err)
			}

			if len(cfg.Services) != len(tt.wantServices) {
				t.Fatalf("got %d services, want %d", len(cfg.Services), len(tt.wantServices))
			}

			for i, wantName := range tt.wantServices {
				svc := cfg.Services[i]
				if svc.Name != wantName {
					t.Errorf("service[%d] name = %q, want %q", i, svc.Name, wantName)
				}
				if wantPort, ok := tt.wantPorts[wantName]; ok {
					if svc.Port != wantPort {
						t.Errorf("service %q port = %d, want %d", wantName, svc.Port, wantPort)
					}
				}
				if wantCmd, ok := tt.wantCommands[wantName]; ok {
					if svc.Command != wantCmd {
						t.Errorf("service %q command = %q, want %q", wantName, svc.Command, wantCmd)
					}
				}
			}
		})
	}
}

func TestPortParsers(t *testing.T) {
	t.Run("properties parser", func(t *testing.T) {
		props := `
# Configuration
server.port = 8082
logging.level.root=INFO
`
		if p := parsePropertiesPort([]byte(props)); p != 8082 {
			t.Errorf("parsePropertiesPort = %d, want 8082", p)
		}

		placeholder := `server.port=${PORT:8089}`
		if p := parsePropertiesPort([]byte(placeholder)); p != 8089 {
			t.Errorf("parsePropertiesPort = %d, want 8089", p)
		}
	})

	t.Run("yaml parser", func(t *testing.T) {
		yml1 := `
server:
  port: 8091
`
		if p := parseYamlPort([]byte(yml1)); p != 8091 {
			t.Errorf("parseYamlPort = %d, want 8091", p)
		}

		yml2 := `
spring:
  application:
    name: test
server:
  servlet:
    context-path: /api
  port: "${PORT:8092}"
`
		if p := parseYamlPort([]byte(yml2)); p != 8092 {
			t.Errorf("parseYamlPort = %d, want 8092", p)
		}

		ymlInline := `server.port: 8093`
		if p := parseYamlPort([]byte(ymlInline)); p != 8093 {
			t.Errorf("parseYamlPort = %d, want 8093", p)
		}
	})
}

func TestLoadOrDetect_MultiCandidate(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `{
		"title": "Custom Reusable App",
		"services": [
			{"name": "API", "command": "go run ./cmd/api", "dir": ".", "port": 8000, "type": "process", "autoStart": true}
		]
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, ".dev-tui.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadOrDetect(tmpDir, "")
	if err != nil {
		t.Fatalf("LoadOrDetect error: %v", err)
	}

	if cfg.Title != "Custom Reusable App" {
		t.Errorf("title = %q, want %q", cfg.Title, "Custom Reusable App")
	}
	if len(cfg.Services) != 1 || cfg.Services[0].Name != "API" {
		t.Fatalf("unexpected services: %+v", cfg.Services)
	}
}

func TestDetectComposeFile_Priority(t *testing.T) {
	t.Run("prioritizes dev prefix over infix and standard", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(""), 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "docker-compose.dev.yml"), []byte(""), 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "dev.docker-compose.yml"), []byte(""), 0644)

		got := DetectComposeFile(tmpDir)
		if got != "dev.docker-compose.yml" {
			t.Errorf("DetectComposeFile = %q, want %q", got, "dev.docker-compose.yml")
		}
	})

	t.Run("prioritizes dev infix over standard", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(""), 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "compose.dev.yaml"), []byte(""), 0644)

		got := DetectComposeFile(tmpDir)
		if got != "compose.dev.yaml" {
			t.Errorf("DetectComposeFile = %q, want %q", got, "compose.dev.yaml")
		}
	})

	t.Run("prioritizes development infix over standard", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(""), 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "docker-compose.development.yml"), []byte(""), 0644)

		got := DetectComposeFile(tmpDir)
		if got != "docker-compose.development.yml" {
			t.Errorf("DetectComposeFile = %q, want %q", got, "docker-compose.development.yml")
		}
	})

	t.Run("falls back to standard compose when no dev compose exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(""), 0644)

		got := DetectComposeFile(tmpDir)
		if got != "docker-compose.yml" {
			t.Errorf("DetectComposeFile = %q, want %q", got, "docker-compose.yml")
		}
	})
}

func TestExtractHostPort(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"3000:3000", 3000},
		{"8080:80/tcp", 8080},
		{"127.0.0.1:5432:5432", 5432},
		{"0.0.0.0:6379:6379/tcp", 6379},
		{"8000-8005:8000-8005", 8000},
		{"3000", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := extractHostPort(tt.input)
		if got != tt.want {
			t.Errorf("extractHostPort(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestInitConfigFile(t *testing.T) {
	t.Run("creates new config with detected services", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = os.MkdirAll(filepath.Join(tmpDir, "server"), 0755)
		_ = os.WriteFile(filepath.Join(tmpDir, "server", "package.json"), []byte(`{"name": "server", "scripts": {"dev": "bun --watch src/index.ts"}}`), 0644)
		_ = os.MkdirAll(filepath.Join(tmpDir, "web"), 0755)
		_ = os.WriteFile(filepath.Join(tmpDir, "web", "package.json"), []byte(`{"name": "web", "scripts": {"dev": "vite"}}`), 0644)

		path, err := InitConfigFile(tmpDir, false)
		if err != nil {
			t.Fatalf("InitConfigFile error: %v", err)
		}

		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected config file at %s to exist: %v", path, err)
		}

		cfg, err := LoadOrDetect(tmpDir, "")
		if err != nil {
			t.Fatalf("LoadOrDetect error: %v", err)
		}

		if len(cfg.Services) != 2 {
			t.Fatalf("expected 2 services in created config, got %d", len(cfg.Services))
		}
		if cfg.Services[0].Name != "Backend" || cfg.Services[0].Dir != "server" {
			t.Errorf("unexpected first service: %+v", cfg.Services[0])
		}
		if cfg.Services[1].Name != "Frontend" || cfg.Services[1].Dir != "web" {
			t.Errorf("unexpected second service: %+v", cfg.Services[1])
		}
	})

	t.Run("fails if config already exists without force", func(t *testing.T) {
		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, ".dev-tui.json"), []byte(`{}`), 0644)

		_, err := InitConfigFile(tmpDir, false)
		if err == nil {
			t.Fatalf("expected error when config file already exists without force")
		}

		// With force, it should overwrite
		_, err = InitConfigFile(tmpDir, true)
		if err != nil {
			t.Fatalf("expected success with force=true, got: %v", err)
		}
	})
}
