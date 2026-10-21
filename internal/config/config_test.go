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
		wantRunner   string
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
			wantRunner: "bun",
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
			wantRunner: "npm",
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
				if cfg.Services[i].Name != wantName {
					t.Errorf("service[%d] name = %q, want %q", i, cfg.Services[i].Name, wantName)
				}
				if wantPort, ok := tt.wantPorts[wantName]; ok {
					if cfg.Services[i].Port != wantPort {
						t.Errorf("service %q port = %d, want %d", wantName, cfg.Services[i].Port, wantPort)
					}
				}
			}
		})
	}
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
