<img width="1910" height="937" alt="image" src="https://github.com/user-attachments/assets/ba597b68-f9d2-4ffc-be24-48d3451e71c2" /><div align="center">


# MediaCLI

Version: **0.1.3** · `mediacli --version`

**An ultra-fast, keyboard-driven terminal media suite wrapping `yt-dlp` and `FFmpeg`.**  
*Implemented in Rust, dual-pane TUI, background task engine, and a searchable command palette.*

---

[![Rust](https://img.shields.io/badge/Rust-stable-dea584?style=flat-square&logo=rust)](https://www.rust-lang.org)
[![AUR Version](https://img.shields.io/aur/version/mediacli-git?color=1793D1&logo=arch-linux&style=flat-square)](https://aur.archlinux.org/packages/mediacli-git)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-blue?style=flat-square)]()

[Key Features](#-key-features) • [Installation](#-installation) • [Keybindings](#-keyboard-shortcuts) • [Architecture](#-under-the-hood)

</div>

---

> [!IMPORTANT]
> **Core Dependencies**: MediaCLI relies on [`yt-dlp`](https://github.com/yt-dlp/yt-dlp) and [`ffmpeg`](https://ffmpeg.org) present in your `$PATH`.

---

## ⚡ Key Features

- [x] **Tabbed TUI Architecture** — Instant keyboard navigation across online downloads, local conversions, and system tools.
- [x] **Command Palette (`Ctrl+P` / `F12`)** — Search actions, open tools, inspect the queue, and change themes from any screen without interrupting active tasks.
- [x] **Non-Blocking Background Queue** — Send running downloads to the background at any time (`q` / `Esc`) and keep browsing.
- [x] **Top-Tier Codec Priority** — Standard MP4 (`H.264+AAC`) and Modern MKV (`AV1/H.265`) always placed first, followed by professional NLE formats (*DaVinci Resolve DNxHR HQ* & *Apple ProRes 422* with uncompressed 16-bit PCM audio).
- [x] **Step-by-Step Manual Wizard** — Granular configuration for resolution limits, custom timecode cuts, dual-audio tracks, and subtitles.
- [x] **Network Acceleration Engine** — Multi-fragment streaming (`--concurrent-fragments`), automated retries, and YouTube anti-throttling mechanisms baked in.
- [x] **SponsorBlock & Chapter Splitting** — Cut sponsor segments on-the-fly or split long podcasts and concerts into track-by-track files.
- [x] **High-Res Poster Extractor** — Standalone mode to fetch original video artwork converted to lossless PNG, JPG, or WEBP.

---

## 🚀 Installation

### npm

Install the terminal application on Linux, macOS, or Windows (x64 and arm64):

```bash
npm install -g @glob876/mediacli
mediacli
```

Node.js 18 or newer starts the program. `yt-dlp`, `ffmpeg`, and `ffprobe` must
be available on `PATH`. The package already contains the prebuilt binaries, so
Rust is not required to install it. The Electron application is distributed
separately.

A build from GitHub Actions can be installed from the saved archive before it
is on the registry:

```bash
npm install -g ./glob876-mediacli-0.1.3.tgz
mediacli --version
```

Publishing and version bumps are described in [npm releases](docs/npm-release.md).

### Arch Linux (AUR)

```bash
# Using paru
paru -S mediacli-git

# Using yay
yay -S mediacli-git
```

### From Source (Any Platform)

```bash
# Clone repository
git clone https://github.com/Glob876/mediacli.git
cd mediacli

# Build and install to PATH
chmod +x build.sh
./build.sh
```

---

## ⌨️ Keyboard Shortcuts

| Key | Context | Action |
| :--- | :--- | :--- |
| <kbd>Ctrl</kbd>+<kbd>P</kbd> / <kbd>F12</kbd> | **Global** | Open the command palette |
| <kbd>←</kbd> <kbd>→</kbd> or <kbd>h</kbd> <kbd>l</kbd> | **Main Menu** | Switch horizontal category tabs |
| <kbd>↑</kbd> <kbd>↓</kbd> or <kbd>k</kbd> <kbd>j</kbd> | **Navigation** | Move cursor selection up / down |
| <kbd>Enter</kbd> | **Dialogs** | Confirm selection / Open sub-menu |
| <kbd>Esc</kbd> / <kbd>q</kbd> | **Runner** | Transfer active task to background queue |
| <kbd>F10</kbd> | **Runner** | Toggle between clean stage view and raw stdout log |
| <kbd>Ctrl</kbd>+<kbd>C</kbd> | **Runner** | Force kill current process |

---

## Command Palette

Press **Ctrl+P** or **F12** from any TUI screen. Type a command name or keyword
in Russian or English, move with **↑/↓**, and press **Enter** to run it.
**Esc**, **Ctrl+P**, or **F12** closes the palette; **Ctrl+U** clears the search.

Actions include downloads, conversion, trimming, pipelines, the universal
editor, the task queue, history, settings, themes, and dependency diagnostics.
Queue actions show logs and cancel active tasks. Clearing history requires
confirmation and retains media files. The previous text console is removed.

---

## CLI 0.1.3

Run without arguments for the TUI, or pass a command for non-interactive use:

```bash
mediacli --help
mediacli download 'https://example.com/video' --quality 1080
mediacli trim input.mp4 --start 00:00:10 --end 00:00:20 -o cut.mp4
mediacli convert input.mp4 --preset standard_mp4 -o output.mp4
mediacli pipeline --project project.json --dry-run
mediacli probe input.mp4 --json
```

Downloads, all media operations, projects, batch processing, size/range exports,
configuration, presets, history, dependency checks and daemon task management are
available as commands. [Full CLI reference and examples (Russian)](docs/cli.md).

## TUI 0.1.3

- The terminal queue is saved in `tui_queue.json`. After a restart, unfinished
  jobs appear as interrupted; use **Ctrl+P → Task queue → Retry whole chain**.
  Downloads reuse yt-dlp's partial files when possible; encodes start again and
  preserve existing output files. Original-replacement conversions retain their
  temporary-file finalization plan.
- **Alt+3** and **Pipeline** autosave separate drafts and offer restoration.
  **Ctrl+Z / Ctrl+Y** undo and redo edits; the same actions are available in menus.
- **Export to size** uses H.264/AAC, decimal MB, a 4% container reserve, and one
  or two passes. The size is approximate. The chosen size and pass mode are saved
  with the editor draft.
- **Ranges / waveform** opens an independent cutting tool. **←/→** move one frame
  according to the source's reported average FPS; **↑/↓** move one second.
  **I** marks a start, **O** appends a range, **D** removes the last range,
  **G** jumps to a time in seconds, **Ctrl+Z / Ctrl+Y** undo/redo range edits.
  **Enter** exports separate clips or joins them in listed order. Accurate cuts
  re-encode; main-editor operations are not applied. The waveform covers the
  current 60-second window, and ranges are autosaved for the current source.
- Batch processing checks inputs, existing destinations and output-name
  collisions before queueing. Review the report, then start valid files.
- History supports search and repeating saved commands. Old records without
  commands remain readable. Original-replacement jobs are repeated from the queue.
  History and queue files use private permissions and atomic replacement;
  concurrent history updates within the app are serialized. History is no longer
  truncated to 50 entries.

The Rust TUI exposes these operations through terminal forms and a file picker.
Electron retains its HTML/CSS interface and compiles from TypeScript.
See [Rust migration](docs/rust-migration.md) for current terminal controls.

---

## ⚙️ Supported Codec Profiles

| Preset ID | Output Container | Video Codec | Audio Codec | Target Scenario |
| :--- | :--- | :--- | :--- | :--- |
| `standard_mp4` | `.mp4` | `H.264 (libx264)` | `AAC (192 kbps)` | Universal playback & web distribution |
| `mkv_av1` | `.mkv` | `AV1 (libsvtav1)` | `AAC / Opus` | Next-Gen high-efficiency compression |
| `hevc_mp4` | `.mp4` | `H.265 (libx265)` | `AAC (192 kbps)` | High quality at 50% reduced file size |
| `davinci_dnxhr` | `.mov` | `DNxHR HQ` | `PCM 16-bit` | DaVinci Resolve editing on Linux/Windows |
| `davinci_prores` | `.mov` | `Apple ProRes 422` | `PCM 16-bit` | Final Cut Pro & DaVinci Master exports |
| `audio_flac` | `.flac` | *None* | `FLAC 24/16-bit` | Lossless audio extraction |

---

## 🔬 Under the Hood

```
+-------------------------------------------------------------+
|                      MediaCLI Core                          |
+-------------------------------------------------------------+
   │                     │                       │
   ▼                     ▼                       ▼
[ ratatui TUI ]    [ Dynamic Parser ]    [ Background Queue ]
(Double-Buffer)   (Regex Stream)        (Tokio Worker Tasks)
   │                     │                       │
   └───────────────┬─────┴───────────────────────┘
                   ▼
     [ Tokio Process Pipeline ]
                   │
         ┌─────────┴─────────┐
         ▼                   ▼
    { yt-dlp }          { FFmpeg }
```

> [!NOTE]
> CLI, TUI и HTTP-бэкенд написаны на Rust. Electron-оболочка использует
> TypeScript. Сборка: `cargo build --release --locked`. Подробности переноса,
> совместимость данных и управление TUI: [Rust migration](docs/rust-migration.md).

---

## 🔌 Daemon API (for the Electron shell)

`mediacli daemon --port 0` starts a loopback-only (`127.0.0.1`) HTTP API over the
same core (queue, history, presets, config) that powers the TUI/GUI:

```bash
MEDIACLI_TOKEN=secret ./mediacli daemon --port 0
# stdout: MEDIACLI_DAEMON_PORT=43721
curl -H "Authorization: Bearer secret" http://127.0.0.1:43721/api/status
```

Endpoints: `GET /api/status`, `POST /api/downloads`, `GET /api/tasks`,
`GET /api/tasks/:id`, `POST /api/tasks/:id/cancel`,
`GET /api/tasks/:id/events` (SSE progress stream), `GET /api/history`,
`DELETE /api/history`, `POST /api/history/delete`, `GET /api/presets`,
`GET /api/convert/presets`, `POST /api/convert` (ffmpeg в общей очереди),
`GET /api/library` (файлы папки загрузок), `GET /api/library/file?name=`
(Range-стрим для плеера, токен через `?token=`),
`GET /api/library/thumb?name=` (JPEG-превью кадра, кеш в `<configDir>/thumbs`),
`GET /api/browse?path=` (обзор ФС для вкладки конвертации),
`GET /api/tools/ffmpeg[?path=]` (какой ffmpeg реально используется + версия),
`GET /api/meta` (справочники для форм: темы, браузеры, пресеты),
`GET/PUT /api/config` (включая `accent_color` и `ffmpeg_path` —
путь уходит в yt-dlp `--ffmpeg-location`, convert и превью),
`POST /api/config/reset` (заводские, как в TUI). Without `MEDIACLI_TOKEN` (or
`--token`) auth is disabled — dev mode only.

## 🖥️ Desktop Shell (Electron, experimental)

`electron/` is a Chromium shell over the daemon API above — no engine logic
is duplicated there; presets/history/config all come from Rust.

```bash
cd electron && npm install
npm start            # needs dist/mediacli built via ./build.sh
npm run dev          # UI-only mock mode, no Rust binary needed
./build.sh --package # AppImage/deb into electron/release/ (--win/--mac on their OS)
```

Shortcut for everything-at-once (build-if-stale, npm ci-if-missing,
Electron download-if-missing, then launch, no sudo needed):

```bash
./run.sh
```

Shell UI: пустая главная с полупрозрачным полем ссылки (кнопка `→` скачивает,
`⚙` открывает пресеты — обе SVG), drag&drop файла в окно открывает Конвертацию;
старт загрузки подсвечивает периметр экрана, слоты заменяют заголовок
(размер — Ctrl+колесо, группировка — очередь строго по одной, состояние
переживает перезапуск, клик по слоту разворачивает живые логи,
двойной клик — логи на весь экран);
настройки — вкладки слева как в TUI (настройки TUI + путь к ffmpeg
с проверкой); при активных загрузках/конвертациях окно не закроется молча —
появится «Вы уверены?» с числом задач; `Shift+H` — история, `Shift+I` — настройки,
`Esc` — закрыть. Языки `ru` / `en` / `en-US`,
акцентный цвет настраивается (`accent_color` в конфиге).

> [!NOTE]
> `yt-dlp` and `ffmpeg` stay external dependencies (checked via the System tab),
> they are not bundled into the installer.

---

## 📄 License

Distributed under the **MIT License**. See [`LICENSE`](LICENSE) for more information.
