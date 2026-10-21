# ⚡ dev-tui

A lightweight, high-performance terminal user interface (TUI) built in Go with **Bubble Tea** and **Lip Gloss** to concurrently run, monitor, and inspect your full-stack developer environment:
- **Backend**: API server logs, status, port listening (Node, Bun, Spring Boot Maven/Gradle).
- **Frontend**: Web dev server (Vite, Next, React, etc.) logs, status, port.
- **Docker Compose**: Real-time container streaming logs and status.
- **Git & Git Worktrees**: Live branch tracking, worktree switcher, and auto-reactivity.

---

## ✨ Features

- 🌿 **Git & Git Worktrees First-Class Support**:
  - Automatically identifies whether your workspace is a standard repository, main worktree, or linked worktree (`.git` pointer file).
  - Displays current branch (`🌿 main`), short commit SHA (`6b3ffca`), worktree name (`⎇ wt:my-feature`), clean/dirty status (`✔` / `*`), and total worktrees count in the header.
  - **Live Auto-Update**: When you switch branches or make commits in another terminal (`git checkout`, `git switch`, `git commit`), `dev-tui` automatically detects the change, updates the header in real-time, and logs a notification in the event stream.
  - **Interactive Worktree Switcher (`[w]`)**: View all worktrees in your repo, navigate with arrow keys, and press `[Enter]` to instantly switch `dev-tui`'s workspace to that worktree without exiting!
- ☕ **Java with Spring Boot**: Native detection for Maven (`./mvnw spring-boot:run` or `mvn spring-boot:run`) and Gradle (`./gradlew bootRun` or `gradle bootRun`), automatically parsing ports from `application.properties`, `application.yml`/`.yaml`, or defaulting to `:8080`.
- 🌐 **Full-Stack & Monorepo Aware**: Detects backends in `backend/`, `server/`, `api/`, or at project root, paired with Node frontend servers (`web/`, `client/`, `frontend/`).
- ⚡ **Live Status & Ports**: Shows real-time service indicators (`● RUNNING`, `◐ STARTING`, `■ STOPPED`, `✖ FAILED`), port mappings (`:8080`, `:3001`, `:5173`), uptime counters, and PIDs.
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

Ensure `~/.local/bin` is in your `$PATH`.

---

## 💻 Usage

In any project or worktree directory, simply run:
```bash
dev-tui
```

Or target a specific directory / worktree:
```bash
dev-tui /path/to/project
dev-tui -dir /path/to/worktree
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

## 🌿 Git & Worktree Workflow

### 1. Live Branch Reactivity
When you are running `dev-tui` and change branches in your external shell:
```bash
git switch feature/auth
```
`dev-tui` automatically detects the change within a second:
- The header updates immediately to `🌿 feature/auth`.
- A log entry is recorded in the feed:
  ```text
  15:40:12 [git] Branch switched: main -> feature/auth (a1b2c3d)
  ```

### 2. Git Worktree Support
If you work with Git worktrees (e.g. for parallel feature branches without stashing):
```bash
# Create a new worktree
git worktree add ../my-app-auth -b feature/auth
```

You can either:
- Run `dev-tui` directly inside the worktree directory:
  ```bash
  cd ../my-app-auth && dev-tui
  ```
  `dev-tui` will detect that it is a linked worktree, display `⎇ wt:my-app-auth [🌿 feature/auth]`, and run services scoped to that worktree.
- Or, if you already have `dev-tui` running in your main repository:
  1. Press **`w`** to open the **Worktrees Modal**.
  2. Use `↑` / `↓` or `k` / `j` to select `../my-app-auth`.
  3. Press **`Enter`** to switch workspace: `dev-tui` stops the previous services, reloads configuration and services for the target worktree, and begins streaming!

---

## ⌨️ Keybindings

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` / `←` / `→` | Switch between service tabs & all-services view |
| `0` | Jump to **All Services** combined view |
| `1` .. `9` | Jump directly to service tab (1: Backend, 2: Frontend, 3: Docker) |
| `w` | Open / close **Worktrees & Git Status** modal |
| `↑` / `↓` (in modal) | Navigate available worktrees |
| `Enter` (in modal) | Switch active workspace to selected worktree |
| `Ctrl+R` | Reload configuration and restart services for current workspace |
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
