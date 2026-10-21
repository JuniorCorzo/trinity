package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/juniorcorzo/dev-tui/internal/config"
)

type Status string

const (
	StatusStopped  Status = "STOPPED"
	StatusStarting Status = "STARTING"
	StatusRunning  Status = "RUNNING"
	StatusFailed   Status = "FAILED"
)

type LogEntry struct {
	Timestamp   time.Time
	ServiceID   int
	ServiceName string
	Text        string
}

const maxLogLines = 2000

type Service struct {
	mu sync.RWMutex

	ID          int
	Name        string
	Command     string
	Dir         string
	Port        int
	Type        config.ServiceType
	ComposeFile string

	Status    Status
	PID       int
	StartTime time.Time
	LastError string

	cmd    *exec.Cmd
	cancel context.CancelFunc

	logs []LogEntry
}

func NewService(id int, cfg config.ServiceConfig, rootDir string) *Service {
	dir := cfg.Dir
	if dir == "" {
		dir = rootDir
	}
	return &Service{
		ID:          id,
		Name:        cfg.Name,
		Command:     cfg.Command,
		Dir:         dir,
		Port:        cfg.Port,
		Type:        cfg.Type,
		ComposeFile: cfg.ComposeFile,
		Status:      StatusStopped,
		logs:        make([]LogEntry, 0, 100),
	}
}

func (s *Service) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

func (s *Service) GetPID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PID
}

func (s *Service) GetUptime() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Status != StatusRunning || s.StartTime.IsZero() {
		return "--"
	}
	dur := time.Since(s.StartTime).Round(time.Second)
	m := int(dur.Minutes())
	sec := int(dur.Seconds()) % 60
	if m > 59 {
		h := m / 60
		m = m % 60
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%02dm:%02ds", m, sec)
}

func (s *Service) GetLogs() []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LogEntry, len(s.logs))
	copy(out, s.logs)
	return out
}

func (s *Service) ClearLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = make([]LogEntry, 0, 100)
}

func (s *Service) appendLog(entry LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.logs) >= maxLogLines {
		s.logs = s.logs[1:]
	}
	s.logs = append(s.logs, entry)
}

// Manager coordinates all services and unified log stream
type Manager struct {
	mu           sync.RWMutex
	Services     []*Service
	LogStream    chan LogEntry
	StatusStream chan struct{}
	allLogs      []LogEntry
	rootDir      string
}

func NewManager(cfg *config.Config, rootDir string) *Manager {
	mgr := &Manager{
		Services:     make([]*Service, len(cfg.Services)),
		LogStream:    make(chan LogEntry, 1000),
		StatusStream: make(chan struct{}, 100),
		allLogs:      make([]LogEntry, 0, 500),
		rootDir:      rootDir,
	}

	for i, sc := range cfg.Services {
		mgr.Services[i] = NewService(i, sc, rootDir)
	}

	return mgr
}

func (m *Manager) GetAllLogs() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]LogEntry, len(m.allLogs))
	copy(out, m.allLogs)
	return out
}

func (m *Manager) AppendAllLog(entry LogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.allLogs) >= maxLogLines {
		m.allLogs = m.allLogs[1:]
	}
	m.allLogs = append(m.allLogs, entry)
}

func (m *Manager) ClearAllLogs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allLogs = make([]LogEntry, 0, 100)
}

func (m *Manager) notifyStatus() {
	select {
	case m.StatusStream <- struct{}{}:
	default:
	}
}

func (m *Manager) StartService(id int) error {
	if id < 0 || id >= len(m.Services) {
		return fmt.Errorf("invalid service id: %d", id)
	}
	s := m.Services[id]

	s.mu.Lock()
	if s.Status == StatusRunning || s.Status == StatusStarting {
		s.mu.Unlock()
		return nil
	}
	s.Status = StatusStarting
	s.LastError = ""
	s.mu.Unlock()
	m.notifyStatus()

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	if s.Type == config.TypeCompose {
		go m.runComposeService(s, ctx)
		return nil
	}

	go m.runProcessService(s, ctx)
	return nil
}

func (m *Manager) runProcessService(s *Service, ctx context.Context) {
	cmd := exec.CommandContext(ctx, "sh", "-c", s.Command)
	cmd.Dir = s.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.mu.Lock()
		s.Status = StatusFailed
		s.LastError = err.Error()
		s.mu.Unlock()
		m.notifyStatus()
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.mu.Lock()
		s.Status = StatusFailed
		s.LastError = err.Error()
		s.mu.Unlock()
		m.notifyStatus()
		return
	}

	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		s.Status = StatusFailed
		s.LastError = err.Error()
		s.mu.Unlock()
		m.notifyStatus()
		return
	}

	s.mu.Lock()
	s.cmd = cmd
	s.PID = cmd.Process.Pid
	s.StartTime = time.Now()
	s.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	go m.streamPipe(s, stdout, &wg)
	go m.streamPipe(s, stderr, &wg)

	if s.Port > 0 {
		go m.probePort(s, ctx)
	} else {
		s.mu.Lock()
		s.Status = StatusRunning
		s.mu.Unlock()
		m.notifyStatus()
	}

	waitErr := cmd.Wait()
	wg.Wait()

	s.mu.Lock()
	s.PID = 0
	if ctx.Err() != nil {
		s.Status = StatusStopped
	} else if waitErr != nil {
		s.Status = StatusFailed
		s.LastError = waitErr.Error()
	} else {
		s.Status = StatusStopped
	}
	s.mu.Unlock()
	m.notifyStatus()
}

func (m *Manager) runComposeService(s *Service, ctx context.Context) {
	composeFileFlag := ""
	if s.ComposeFile != "" {
		composeFileFlag = fmt.Sprintf("-f %s", s.ComposeFile)
	}
	upCmd := fmt.Sprintf("docker compose %s up -d", composeFileFlag)
	s.appendLog(LogEntry{
		Timestamp:   time.Now(),
		ServiceID:   s.ID,
		ServiceName: s.Name,
		Text:        fmt.Sprintf("Running: %s...", upCmd),
	})

	initCmd := exec.CommandContext(ctx, "sh", "-c", upCmd)
	initCmd.Dir = s.Dir
	_ = initCmd.Run()

	s.mu.Lock()
	s.Status = StatusRunning
	s.StartTime = time.Now()
	s.mu.Unlock()
	m.notifyStatus()

	logsCmdStr := fmt.Sprintf("docker compose %s logs -f --tail=100", composeFileFlag)
	cmd := exec.CommandContext(ctx, "sh", "-c", logsCmdStr)
	cmd.Dir = s.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err == nil {
		stderr, err2 := cmd.StderrPipe()
		if err2 == nil {
			if err := cmd.Start(); err == nil {
				s.mu.Lock()
				s.cmd = cmd
				s.PID = cmd.Process.Pid
				s.mu.Unlock()

				var wg sync.WaitGroup
				wg.Add(2)
				go m.streamPipe(s, stdout, &wg)
				go m.streamPipe(s, stderr, &wg)

				_ = cmd.Wait()
				wg.Wait()
			}
		}
	}

	s.mu.Lock()
	s.PID = 0
	s.Status = StatusStopped
	s.mu.Unlock()
	m.notifyStatus()
}

func (m *Manager) streamPipe(s *Service, r io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		entry := LogEntry{
			Timestamp:   time.Now(),
			ServiceID:   s.ID,
			ServiceName: s.Name,
			Text:        text,
		}
		s.appendLog(entry)
		m.AppendAllLog(entry)
		select {
		case m.LogStream <- entry:
		default:
		}
	}
}

func (m *Manager) probePort(s *Service, ctx context.Context) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	target := fmt.Sprintf("127.0.0.1:%d", s.Port)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			status := s.Status
			s.mu.RUnlock()
			if status != StatusStarting {
				return
			}

			conn, err := net.DialTimeout("tcp", target, 250*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				s.mu.Lock()
				s.Status = StatusRunning
				s.mu.Unlock()
				m.notifyStatus()
				return
			}
		}
	}
}

func (m *Manager) StopService(id int) error {
	if id < 0 || id >= len(m.Services) {
		return fmt.Errorf("invalid service id: %d", id)
	}
	s := m.Services[id]

	s.mu.Lock()
	if s.Status == StatusStopped {
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	cmd := s.cmd
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		go func(pid int) {
			time.Sleep(1 * time.Second)
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}(cmd.Process.Pid)
	}

	if s.Type == config.TypeCompose {
		composeFileFlag := ""
		if s.ComposeFile != "" {
			composeFileFlag = fmt.Sprintf("-f %s", s.ComposeFile)
		}
		stopCmd := exec.Command("sh", "-c", fmt.Sprintf("docker compose %s stop", composeFileFlag))
		stopCmd.Dir = s.Dir
		_ = stopCmd.Run()
	}

	s.mu.Lock()
	s.Status = StatusStopped
	s.PID = 0
	s.mu.Unlock()
	m.notifyStatus()

	return nil
}

func (m *Manager) RestartService(id int) error {
	if err := m.StopService(id); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond)
	return m.StartService(id)
}

func (m *Manager) StartAll() {
	for i := range m.Services {
		_ = m.StartService(i)
	}
}

func (m *Manager) StopAll() {
	for i := range m.Services {
		_ = m.StopService(i)
	}
}

func (m *Manager) RestartAll() {
	m.StopAll()
	time.Sleep(500 * time.Millisecond)
	m.StartAll()
}

func (m *Manager) GetRootDir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rootDir
}

func (m *Manager) ResetServices(cfg *config.Config, rootDir string) {
	m.StopAll()
	m.mu.Lock()
	m.rootDir = rootDir
	m.Services = make([]*Service, len(cfg.Services))
	for i, sc := range cfg.Services {
		m.Services[i] = NewService(i, sc, rootDir)
	}
	m.mu.Unlock()
	m.notifyStatus()
}


func CleanServiceName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "spring") || strings.Contains(lower, "java"):
		return "spring"
	case strings.Contains(lower, "back"):
		return "backend"
	case strings.Contains(lower, "front") || strings.Contains(lower, "web"):
		return "frontend"
	case strings.Contains(lower, "compose") || strings.Contains(lower, "docker"):
		return "compose"
	default:
		return lower
	}
}
