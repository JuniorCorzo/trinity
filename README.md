```text
  _        _       _ _              _ _ 
 | |_ _ __(_)_ __ (_) |_ _   _  ___| (_)
 | __| '__| | '_ \| | __| | | |/ __| | |
 | |_| |  | | | | | | |_| |_| | (__| | |
  \__|_|  |_|_| |_|_|\__|\__, |\___|_|_|
                         |___/          
  [DB] ── [API] ── [WEB]  ·  v0.1.0
```

> **trinitycli** — Lightweight terminal orchestrator for fullstack monorepos. Concurrently run, monitor, and inspect your database, backend, and frontend environments in a single TUI.

---

## ✨ Features

- 🌿 **Git & Git Worktrees First-Class Support**:
  - Automatically identifies whether your workspace is a standard repository, main worktree, or linked worktree (`.git` pointer file).
  - Displays current branch (`🌿 main`), short commit SHA (`6b3ffca`), worktree name (`⎇ wt:my-feature`), clean/dirty status (`✔` / `*`), and total worktrees count in the header.
  - **Live Auto-Update**: When you switch branches or make commits in another terminal (`git checkout`, `git switch`, `git commit`), `trinitycli` automatically detects the change, updates the header in real-time, and logs a notification in the event stream.
  - **Interactive Worktree Switcher (`[w]`)**: View all worktrees in your repo, navigate with arrow keys, and press `[Enter]` to instantly switch `trinitycli`'s workspace to that worktree without exiting!
- ☕ **Java with Spring Boot**: Native detection for Maven (`./mvnw spring-boot:run` or `mvn spring-boot:run`) and Gradle (`./gradlew bootRun` or `gradle bootRun`), automatically parsing ports from `application.properties`, `application.yml`/`.yaml`, or defaulting to `:8080`.
- 🌐 **Full-Stack & Monorepo Aware**: Detects backends in `backend/`, `server/`, `api/`, or at project root, paired with Node frontend servers (`web/`, `client/`, `frontend/`).
- ⚡ **Live Status & Ports**: Shows real-time service indicators (`● RUNNING`, `◐ STARTING`, `■ STOPPED`, `✖ FAILED`), port mappings (`:8080`, `:3001`, `:5173`), uptime counters, and PIDs.
- 📜 **Log Streaming**: Unified all-in-one view with colored service tags, plus dedicated single-service tabs.
- 🔄 **Safe Process Termination**: Uses Linux process groups (`Setpgid`) so shutting down or restarting cleanly terminates the process tree (preventing orphaned Java, JVM, Bun, Node, Vite, or Esbuild processes).
- 🐳 **Docker Compose First-Class Support**:
  - **Dev-First Prioritization**: Automatically prioritizes development compose files (`dev.docker-compose.yml`, `compose.dev.yaml`, `docker-compose.dev.yml`, `dev-compose.yml`, `docker-compose.development.yaml`) over production configurations.
  - **Granular Service Discovery**: Parses all declared services (`api`, `web`, `db`, `redis`) directly into dedicated interactive cards with individual start, stop, restart, and log filtering.
  - **Accurate Host Port Extraction**: Reads published host ports directly from Compose mappings (`3000:3000`, `127.0.0.1:8080:80`, long-syntax `published: 5432`), displaying true container endpoints without guessing or relying on local start scripts.
- 🔍 **Zero-Config Auto-Detection**: Discovers Maven/Gradle (Spring Boot), Bun/NPM/Yarn/PNPM workspaces, and Docker Compose out-of-the-box.
- ⚙️ **Configurable**: Fully customizable via `.trinity.json`, `.dev-tui.json`, or `--config <file>`.

---

## 🚀 Installation

```bash
# Clone and install locally into ~/.local/bin:
cd dev-tui
make install
```

Or build with Go directly:
```bash
go install github.com/juniorcorzo/dev-tui/cmd/trinitycli@latest
```

Ensure `~/.local/bin` is in your `$PATH`.

---

## 💻 Usage

In any project or worktree directory, simply run:
```bash
trinitycli
```

### Initializing Configuration (`.trinity.json`)
To generate or customize project configuration, run:
```bash
trinitycli init
# Or overwrite an existing configuration:
trinitycli init --force
```

This creates a `.trinity.json` file in the project root:
```json
{
  "title": "my-project",
  "services": [
    {
      "name": "Backend",
      "dir": "server",
      "command": "bun run dev",
      "port": 3001,
      "type": "process",
      "autoStart": true
    },
    {
      "name": "Frontend",
      "dir": "web",
      "command": "bun run dev",
      "port": 5173,
      "type": "process",
      "autoStart": true
    }
  ]
}
```

Or target a specific directory / worktree with flags:
```bash
trinitycli /path/to/project
trinitycli -dir /path/to/worktree
trinitycli -config /path/to/custom-config.json
trinitycli -v           # Print version, commit, and build date
trinitycli -debug        # Log internal diagnostics to .trinity.log
NO_COLOR=1 trinitycli   # Respects NO_COLOR standard for accessible monochrome terminals
```

### Integration with `package.json`

Add `trinitycli` to your `package.json` scripts:
```json
{
  "scripts": {
    "dev": "trinitycli",
    "dev:tui": "trinitycli"
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
When you are running `trinitycli` and change branches in your external shell:
```bash
git switch feature/auth
```
`trinitycli` automatically detects the change within a second:
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
- Run `trinitycli` directly inside the worktree directory:
  ```bash
  cd ../my-app-auth && trinitycli
  ```
  `trinitycli` will detect that it is a linked worktree, display `⎇ wt:my-app-auth [🌿 feature/auth]`, and run services scoped to that worktree.
- Or, if you already have `trinitycli` running in your main repository:
  1. Press **`w`** to open the **Worktrees Modal**.
  2. Use `↑` / `↓` or `k` / `j` to select `../my-app-auth`.
  3. Press **`Enter`** to switch workspace: `trinitycli` stops the previous services, reloads configuration and services for the target worktree, and begins streaming!

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
| `v` / `V` | Enter **Neovim Visual / Line Copy mode** (`v`: character, `V`: line-wise) |
| `h` / `j` / `k` / `l` (in copy mode) | Move cursor and expand selection |
| `w` / `b` (in copy mode) | Jump forward / backward by word |
| `0` / `$` (in copy mode) | Jump to start / end of line |
| `gg` / `G` (in copy mode) | Jump to top / bottom of log buffer |
| `Ctrl+u` / `Ctrl+d` (in copy mode) | Half-page scroll up / down |
| `y` (in copy mode) | **Yank** (copy) selected logs to clipboard via OSC 52 |
| `Esc` / `q` (in copy mode) | Cancel selection and exit visual copy mode |
| `↑` / `↓` / `PgUp` / `PgDown` | Scroll log viewport |
| `q` / `Ctrl+C` | Gracefully terminate all processes and exit |

---

## 🔧 Configuration (`.trinity.json`)

If your project requires custom commands or non-standard folders, create `.trinity.json` (or `.dev-tui.json`) in the root of your project:

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
