package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/juniorcorzo/dev-tui/internal/config"
	"github.com/juniorcorzo/dev-tui/internal/service"
	"github.com/juniorcorzo/dev-tui/internal/theme"
	"github.com/juniorcorzo/dev-tui/internal/ui"
)

var (
	Version   = "0.1.0"
	Commit    = "dev"
	BuildDate = "unknown"
)

func usage(w io.Writer) {
	fmt.Fprintf(w, `trinity - Lightweight Full-Stack Terminal Orchestrator

USAGE:
  trinity [flags] [directory]
  trinity init [directory] [--force]
  trinity theme list
  trinity theme install <path|url>

FLAGS:
  -dir string
        Target project directory (default: current directory)
  -config string
        Path to custom configuration file (.json)
  -theme string
        Theme name (default: terminal, options: terminal, catppuccin-noir, dracula, tokyo-night)
  -init, --init
        Generate a starter .trinity.json configuration file
  -force, --force
        Overwrite existing configuration file when initializing
  -debug
        Enable debug logging to .trinity.log
  -v, --version
        Print version information
  -h, --help
        Show this help message

KEYBOARD SHORTCUTS:
  b                 Toggle sidebar mode (Expanded / Compact / Hidden)
  Tab / Shift+Tab   Switch active service tabs (or Left/Right, h/l)
  0-9               Direct jump to "All Services" (0) or specific service (1-9)
  r                 Restart currently selected service (or all if on tab 0)
  R                 Restart all services
  s                 Toggle Start/Stop for active service (or all)
  w                 Open Git Worktrees & Branch switcher modal
  c                 Clear logs in current tab
  a                 Toggle log auto-scroll
  v / V             Enter Neovim Visual / Line Copy mode (y: yank, Esc/q: cancel)
  q / Ctrl+C        Stop all running services and quit

CONFIGURATION:
  trinity auto-detects reactive native services (server, backend, web, frontend)
  for Node.js (bun/npm/pnpm/yarn), Spring Boot (Maven/Gradle), and Go, plus
  auxiliary docker-compose infrastructure.

  To create or customize a configuration file, run:
    trinity init
`)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	isInitSubcommand := false
	isThemeSubcommand := false
	var themeArgs []string

	if len(args) > 0 {
		switch args[0] {
		case "init":
			isInitSubcommand = true
			args = args[1:]
		case "theme":
			isThemeSubcommand = true
			themeArgs = args[1:]
			args = args[1:]
		}
	}

	fs := flag.NewFlagSet("trinity", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(stderr) }

	dirFlag := fs.String("dir", "", "Target project directory")
	configFlag := fs.String("config", "", "Path to custom configuration file (.json)")
	themeFlag := fs.String("theme", "", "Active theme name")
	initFlag := fs.Bool("init", false, "Generate starter .trinity.json configuration file")
	forceFlag := fs.Bool("force", false, "Overwrite existing configuration file")
	debugFlag := fs.Bool("debug", false, "Enable debug logging to .trinity.log")
	versionFlag := fs.Bool("v", false, "Print trinity version")
	versionLongFlag := fs.Bool("version", false, "Print trinity version")

	if !isThemeSubcommand {
		if err := fs.Parse(args); err != nil {
			if err == flag.ErrHelp {
				return 0
			}
			return 2
		}
	}

	if *versionFlag || *versionLongFlag {
		fmt.Fprintf(stdout, "trinity v%s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
		return 0
	}

	targetDir := "."
	if *dirFlag != "" {
		targetDir = *dirFlag
	} else if fs.NArg() > 0 && !isThemeSubcommand {
		targetDir = fs.Arg(0)
	}

	absRoot, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving project directory: %v\n", err)
		return 1
	}

	if isThemeSubcommand {
		if len(themeArgs) == 0 || themeArgs[0] == "list" {
			themes := theme.ListThemes(absRoot)
			fmt.Fprintln(stdout, "Available Trinity Themes:")
			for _, th := range themes {
				mode := "solid"
				if th.Transparent {
					mode = "transparent"
				}
				fmt.Fprintf(stdout, "  • %-16s [%-11s] %s\n", th.Name, mode, th.Description)
			}
			return 0
		}
		if themeArgs[0] == "install" {
			if len(themeArgs) < 2 {
				fmt.Fprintln(stderr, "Usage: trinity theme install <path|url>")
				return 1
			}
			th, path, err := theme.InstallTheme(themeArgs[1])
			if err != nil {
				fmt.Fprintf(stderr, "Error installing theme: %v\n", err)
				return 1
			}
			fmt.Fprintf(stdout, "✓ Installed theme %q to %s\n", th.Name, path)
			return 0
		}
		fmt.Fprintf(stderr, "Unknown theme command %q. Use 'list' or 'install'.\n", themeArgs[0])
		return 1
	}

	if isInitSubcommand || *initFlag {
		createdPath, err := config.InitConfigFile(absRoot, *forceFlag)
		if err != nil {
			fmt.Fprintf(stderr, "Error initializing config: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "✓ Created configuration file: %s\n\nYou can edit this file to configure:\n  - dir: Directory where the process runs\n  - command: Command to run\n  - port: Port for status health-checks\n  - autoStart: Whether to launch automatically\n  - theme: Theme name ('terminal', 'catppuccin-noir', 'dracula', 'tokyo-night')\n", createdPath)
		return 0
	}

	if *debugFlag || os.Getenv("DEV_TUI_DEBUG") != "" {
		logPath := filepath.Join(absRoot, ".trinity.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			defer logFile.Close()
			handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})
			slog.SetDefault(slog.New(handler))
			slog.Info("trinity started", "version", Version, "root", absRoot)
		}
	}

	cfg, err := config.LoadOrDetect(absRoot, *configFlag)
	if err != nil {
		fmt.Fprintf(stderr, "Error loading configuration: %v\n", err)
		return 1
	}

	if *themeFlag != "" {
		cfg.Theme = *themeFlag
	}

	if len(cfg.Services) == 0 {
		fmt.Fprintf(stderr, "No services found in %s.\nPlease create .trinity.json or run inside a project with backend/frontend/docker-compose.\n", absRoot)
		return 1
	}

	mgr := service.NewManager(cfg, absRoot)

	// Auto-start marked services
	for i, sc := range cfg.Services {
		if sc.AutoStart {
			_ = mgr.StartService(i)
		}
	}

	if os.Getenv("NO_COLOR") == "" {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}

	// Resolve and apply theme
	activeTheme, _ := theme.LoadTheme(cfg.Theme, absRoot)
	if activeTheme == nil {
		activeTheme = &theme.TerminalTheme
	}
	ui.ApplyTheme(activeTheme)

	if !activeTheme.Transparent && activeTheme.Bg != "" && os.Getenv("NO_COLOR") == "" {
		fmt.Fprintf(stdout, "\x1b]11;%s\x07", activeTheme.Bg)
		defer fmt.Fprint(stdout, "\x1b]111\x07")
	}

	model := ui.NewModel(cfg, mgr)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Setup graceful signal context
	sigCtx, stopSig := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stopSig()

	go func() {
		<-sigCtx.Done()
		slog.Debug("shutdown signal received, stopping services")
		mgr.StopAll()
		p.Kill()
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(stderr, "trinity runtime error: %v\n", err)
		mgr.StopAll()
		return 1
	}

	stopSig()
	mgr.StopAll()
	return 0
}
