# Brand Context & Identity Brief — trinitycli

**Brand Name:** `trinitycli`  
**Concept:** Lightweight terminal orchestrator for fullstack monorepos (Database + API + Frontend).

## 01 — IDENTITY STRATEGY STATEMENT
A minimalist, terminal-first visual identity engineered for `trinitycli`. Rooted in the functional austerity of Vim and the spatial utility of Lazygit, this identity rejects marketing fluff and decoration in favor of keyboard rhythm, density, and immediate clarity. Set against a Catppuccin Noir OLED backdrop with restrained functional accents for the three services (DB, API, Web), it projects speed, simplicity, and unpretentious open-source craftsmanship.

---

## 02 — LOGO DIRECTION
* **Primary direction:** Monospaced typographic wordmark paired with a modular Unicode/ASCII geometric glyph (representing concurrent orchestration of DB, API, and UI).
* **Character:** Utilitarian, fast, engineered, quiet, keyboard-centric.
* **Style notes:** Monospaced, geometric, zero border radiuses, sharp terminal vector aesthetic.
* **What to avoid:** Gradients, rounded bubbly friendly shapes, cartoon mascots, isometric 3D renders, cloud/gear clichés.
* **3 Logo References:**
  1. **Vim (`V` block mark):** Raw geometric economy and unmistakable silhouette that works in 16x16px or monochrome ASCII.
  2. **Lazygit:** Clean boxed-panel framing, border syntax, and terminal-native simplicity.
  3. **Charm.sh (Bubbletea / Gum):** Restrained typography and high-craft developer tooling polish.

---

## 03 — COLOR PALETTE (Catppuccin Noir)
* **Palette Type:** Monochromatic base with functional triadic accents.
* **Base Background (Crust / OLED):** `#000000` — Pure black canvas for maximum terminal contrast.
* **Surface Background (Mantle):** `#0f0f14` — Deep night surface for inactive panes and panels.
* **Borders / Separators (Surface0):** `#252532` — Subtle dark gray line structure.
* **Text Primary (Text):** `#cdd6f4` — Crisp off-white for high legibility without glare.
* **Text Muted (Subtext):** `#6c7086` — Medium slate for secondary metadata and hotkey hints.
* **Accent 1 — Active / Database (Green):** `#a6e3a1` — Vim-inspired emerald for healthy running states and database process.
* **Accent 2 — API / Gateway (Mauve / Lavender):** `#cba6f7` — Distinctive Catppuccin pastel for backend/API logs.
* **Accent 3 — Frontend / UI (Sapphire / Sky):** `#74c7ec` — Clean terminal cyan for web/client services.
* **Color Personality:** Nocturnal, calm, precise, non-fatiguing for extended terminal sessions.
* **Usage Rules:** 85% black/slate monochrome. Colors are strictly state indicators, never ornamental.

---

## 04 — TYPOGRAPHY
* **Primary Typeface:** `JetBrains Mono` (Open Source, SIL OFL) — designed for code legibility, high x-height, clear ligatures and box-drawing alignment.
* **Secondary / Docs Typeface:** `Geist Mono` or `Inter` (for web documentation fallbacks).
* **Type Personality:** Technical precision, tabular alignment, monospaced rhythm.
* **Usage Hierarchy:**
  * **Brand Mark / ASCII Banner:** Monospace block / ANSI figlet font.
  * **Headings / Status:** `UPPERCASE MONO`, semi-bold, subtle letter spacing.
  * **Body / Logs / Keybinds:** Regular monospace, tabular figures enabled.
* **Avoid:** Serif fonts, handwriting/brush fonts, geometric sans-serifs without monospace variants.

---

## 05 — IMAGERY & MEDIA STYLE
* **Overall aesthetic:** Authentic terminal screen recordings (VHS / asciinema), high-contrast SVG terminal mockups, crisp dark-mode diagrams.
* **Subject matter:** Live running processes, split-pane terminal dashboards, terminal keybindings in action, minimal config files.
* **Color treatment:** OLED pure blacks with subtle glowing borders; natural uncompressed terminal captures.
* **What to avoid:** Stock photos of people holding laptops, blurred glassmorphism gradients, generic SaaS vector illustrations.

---

## 06 — ICONOGRAPHY & GLYPHS
* **Style:** Terminal Unicode glyphs (`│`, `┌`, `└`, `■`, `●`, `➜`) and Nerd Fonts icons.
* **Weight:** Single-width line weight matching standard terminal box-drawing characters.
* **Personality:** Strict functional ergonomics; iconography functions as status telemetry, not decoration.

---

## 07 — DESIGN PRINCIPLES
1. **Utility Precedes Form:** Every pixel, character, and border must convey system status or navigation hierarchy.
2. **Terminal-Native First:** The brand must render natively in an 80x24 ANSI terminal without degrading.
3. **Restrained Nocturnal Contrast:** Reserve color strictly for runtime process status (DB, API, Front); the rest remains OLED black and slate.
4. **Zero Latency Aura:** Copy, banners, and layouts must feel lightweight and instantly readable.

---

## 08 — BRAND TOUCHPOINTS
* **Terminal TUI Interface:** Boxed layout with Catppuccin Noir borders, clean header bar showing `[DB] [API] [WEB]` states, bottom hotkey bar.
* **GitHub README:** Clean ANSI-art title banner, SVG status badges (black + mauve/green accents), and a compact 10-second asciinema/VHS demo.
* **CLI Banner (`--help` / version):**
```text
  _        _       _ _              _ _ 
 | |_ _ __(_)_ __ (_) |_ _   _  ___| (_)
 | __| '__| | '_ \| | __| | | |/ __| | |
 | |_| |  | | | | | | |_| |_| | (__| | |
  \__|_|  |_|_| |_|_|\__|\__, |\___|_|_|
                         |___/          
  [DB] ── [API] ── [WEB]  ·  v0.1.0
```
* **Social / Open-Source Preview Cards:** Minimalist black card featuring the glyph mark, one terminal snippet, and the repo link.
