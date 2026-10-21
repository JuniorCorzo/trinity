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
			name: "detects backend, frontend and docker compose with bun",
			setupFiles: map[string]string{
				"bun.lock":            "",
				"server/package.json": `{"name": "server"}`,
				"server/.env":         "PORT=3001\n",
				"web/package.json":    `{"name": "web"}`,
				"web/.env":            "PORT=5173\n",
				"docker-compose.yml":  "services:\n  app:\n    ports:\n      - \"8080:8080\"\n",
			},
			wantServices: []string{"Docker Compose", "Backend", "Frontend"},
			wantPorts: map[string]int{
				"Docker Compose": 8080,
				"Backend":        3001,
				"Frontend":       5173,
			},
			wantCommands: map[string]string{
				"Backend":  "bun run dev",
				"Frontend": "bun run dev",
			},
		},
		{
			name: "detects java spring boot maven backend with mvnw and properties port",
			setupFiles: map[string]string{
				"backend/pom.xml":                                  `<project><groupId>com.example</groupId><artifactId>demo</artifactId><parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId><version>3.2.0</version></parent></project>`,
				"backend/mvnw":                                     `#!/bin/sh`,
				"backend/src/main/resources/application.properties": "server.port=8085\n",
				"frontend/package.json":                            `{"name": "frontend"}`,
				"docker-compose.yml":                               "services:\n  db:\n    ports:\n      - \"5432:5432\"\n",
			},
			wantServices: []string{"Docker Compose", "Backend", "Frontend"},
			wantPorts: map[string]int{
				"Docker Compose": 5432,
				"Backend":        8085,
				"Frontend":       5173,
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
			name: "detects npm root dev fallback if no subdirectories",
			setupFiles: map[string]string{
				"package.json": `{"name": "single-app"}`,
			},
			wantServices: []string{"Dev"},
			wantPorts: map[string]int{
				"Dev": 3000,
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
