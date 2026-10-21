# ⚡ dev-tui

A lightweight, high-performance terminal user interface (TUI) built in Go with **Bubble Tea** and **Lip Gloss** to concurrently run, monitor, and inspect your full-stack developer environment:
- **Backend**: API server logs, status, port listening.
- **Frontend**: Web dev server (Vite, Next, React, etc.) logs, status, port.
- **Docker Compose**: Real-time container streaming logs and status.

---

## ✨ Features

- 🟢 **Java with Spring Boot**: Native detection for Maven (`./mvnw spring-boot:run` or `mvn spring-boot:run`) and Gradle (`./gradlew bootRun` or `gradle bootRun`), automatically parsing ports from `application.properties`, `application.yml`/`.yaml`, or defaulting to `:8080`.
- 🟢 **Full-Stack & Monorepo Aware**: Detects Spring Boot backends in `backend/`, `server/`, `api/`, or at project root, paired with Node frontend servers (`web/`, `client/`, `frontend/`).
- 🟢 **Live Status & Ports**: Shows real-time service indicators (`● RUNNING`, `◐ STARTING`, `■ STOPPED`, `✖ FAILED`), port mappings (`:8080`, `:3001`, `:5173`), uptime counters, and PIDs.
- 📜 **Log Streaming**: Unified all-in-one view with colored service tags, plus dedicated single-service tabs.
- 🔄 **Safe Process Termination**: Uses Linux process groups (`Setpgid`) so shutting down or restarting cleanly terminates the process tree (preventing orphaned Java, JVM, Bun, Node, Vite, or Esbuild processes).
- 🐳 **Docker Compose Support**: Automatically hooks into `docker-compose.yml` or `compose.yaml` to stream container logs (`docker compose logs -f --tail=100`).
- 🔍 **Zero-Config Auto-Detection**: Discovers Maven/Gradle (Spring Boot), Bun/NPM/Yarn/PNPM workspaces, and Docker Compose out-of-the-box.
- ⚙️ **Configurable**: Fully customizable via `.dev-tui.json`, `dev-tui.json`, or `--config <file>`.

---

## 🚀 Installation

```bash
# Clone and install locally into ~/.local/bin:
cd dev-tui
make install
```

Or build with Go directly:
```bash
go install github.com/juniorcorzo/dev-tui/cmd/dev-tui@latest
```

---

## 💻 Usage

In any project directory, simply run:
```bash
dev-tui
```

Or target a specific directory:
```bash
dev-tui /path/to/project
dev-tui -dir /path/to/project
```

### Integration with `package.json`

Add `dev-tui` to your `package.json` scripts:
```json
{
  "scripts": {
    "dev": "dev-tui",
    "dev:tui": "dev-tui"
  }
}
```

Then run with your package manager:
```bash
bun run dev:tui
# or
npm run dev:tui
```

---

## ⌨️ Keybindings

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` / `←` / `→` | Switch between service tabs & all-services view |
| `0` | Jump to **All Services** combined view |
| `1` .. `9` | Jump directly to service tab |
| `r` | Restart active service |
| `R` | Restart all services |
| `s` | Toggle start / stop for active service (or all on tab 0) |
| `c` | Clear logs for active tab |
| `a` | Toggle auto-scroll (paused when scrolling up) |
| `↑` / `↓` / `PgUp` / `PgDown` | Scroll log viewport |
| `q` / `Ctrl+C` | Gracefully terminate all processes and exit |

---

## 🔧 Configuration (`.dev-tui.json`)

If your project requires custom commands or non-standard folders, create `.dev-tui.json` in the root of your project:

```json
{
  "title": "My Project",
  "services": [
    {
      "name": "Backend",
      "command": "bun run dev",
      "dir": "server",
      "port": 3001,
      "type": "process",
      "autoStart": true
    },
    {
      "name": "Frontend",
      "command": "bun run dev",
      "dir": "web",
      "port": 5173,
      "type": "process",
      "autoStart": true
    },
    {
      "name": "Docker Compose",
      "command": "docker compose logs -f --tail=100",
      "dir": ".",
      "composeFile": "docker-compose.yml",
      "port": 5432,
      "type": "compose",
      "autoStart": false
    }
  ]
}
```

---

## 🧪 Testing

```bash
make test
```
