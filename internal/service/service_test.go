package service

import (
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/juniorcorzo/dev-tui/internal/config"
)

func TestService_LogRingBuffer(t *testing.T) {
	defer goleak.VerifyNone(t)
	s := NewService(0, config.ServiceConfig{Name: "Test"}, "/tmp")

	for i := 0; i < 10; i++ {
		s.appendLog(LogEntry{
			Timestamp:   time.Now(),
			ServiceID:   0,
			ServiceName: "Test",
			Text:        "log line",
		})
	}

	logs := s.GetLogs()
	if len(logs) != 10 {
		t.Fatalf("expected 10 logs, got %d", len(logs))
	}

	s.ClearLogs()
	if len(s.GetLogs()) != 0 {
		t.Fatalf("expected 0 logs after clear, got %d", len(s.GetLogs()))
	}
}

func TestManager_Lifecycle(t *testing.T) {
	defer goleak.VerifyNone(t)
	if testing.Short() {
		t.Skip("skipping process execution test in short mode")
	}

	cfg := &config.Config{
		Title: "TestManager",
		Services: []config.ServiceConfig{
			{
				Name:      "Echoer",
				Command:   "echo 'hello world' && sleep 0.5",
				Type:      config.TypeProcess,
				AutoStart: true,
			},
		},
	}

	mgr := NewManager(cfg, "/tmp")
	if len(mgr.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(mgr.Services))
	}

	err := mgr.StartService(0)
	if err != nil {
		t.Fatalf("StartService error: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	logs := mgr.Services[0].GetLogs()
	if len(logs) == 0 {
		t.Errorf("expected at least 1 log entry, got none")
	}

	_ = mgr.StopService(0)
	if mgr.Services[0].GetStatus() != StatusStopped {
		t.Errorf("expected status STOPPED, got %v", mgr.Services[0].GetStatus())
	}
}

func TestCleanServiceName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Backend API", "backend"},
		{"Frontend Web", "frontend"},
		{"Web UI", "frontend"},
		{"Docker Compose", "compose"},
		{"Spring Boot", "spring"},
		{"Java Service", "spring"},
		{"MyCustomService", "mycustomservice"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := CleanServiceName(tt.input)
			if got != tt.want {
				t.Errorf("CleanServiceName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
