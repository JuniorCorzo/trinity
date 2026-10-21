package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorcorzo/dev-tui/internal/config"
	"github.com/juniorcorzo/dev-tui/internal/service"
	"github.com/juniorcorzo/dev-tui/internal/ui"
)

var Version = "0.1.0"

func main() {
	dirFlag := flag.String("dir", "", "Target project directory (default: current directory)")
	configFlag := flag.String("config", "", "Path to custom configuration file (.json)")
	versionFlag := flag.Bool("v", false, "Print dev-tui version")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("dev-tui v%s\n", Version)
		os.Exit(0)
	}

	targetDir := "."
	if *dirFlag != "" {
		targetDir = *dirFlag
	} else if flag.NArg() > 0 {
		targetDir = flag.Arg(0)
	}

	absRoot, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving project directory: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadOrDetect(absRoot, *configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Services) == 0 {
		fmt.Fprintf(os.Stderr, "No services found in %s.\nPlease create .dev-tui.json or run inside a project with backend/frontend/docker-compose.\n", absRoot)
		os.Exit(1)
	}

	mgr := service.NewManager(cfg, absRoot)

	// Auto-start marked services
	for i, sc := range cfg.Services {
		if sc.AutoStart {
			_ = mgr.StartService(i)
		}
	}

	model := ui.NewModel(cfg, mgr)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "dev-tui runtime error: %v\n", err)
		mgr.StopAll()
		os.Exit(1)
	}

	mgr.StopAll()
}
