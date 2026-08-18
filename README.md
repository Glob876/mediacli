<img width="1910" height="937" alt="image" src="https://github.com/user-attachments/assets/ba597b68-f9d2-4ffc-be24-48d3451e71c2" /><div align="center">


# MediaCLI

**An ultra-fast, keyboard-driven terminal media suite wrapping `yt-dlp` and `FFmpeg`.**  
*Engineered in Go with zero runtime overhead, dual-pane TUI, background task engine, and an interactive overlay terminal.*

---

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://golang.org)
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
- [x] **Global Floating Terminal (`F12` / `Alt+Shift+P`)** — Drop-down command console accessible from any screen without interrupting active tasks.
- [x] **Non-Blocking Background Queue** — Send running downloads to the background at any time (`q` / `Esc`) and keep browsing.
- [x] **Top-Tier Codec Priority** — Standard MP4 (`H.264+AAC`) and Modern MKV (`AV1/H.265`) always placed first, followed by professional NLE formats (*DaVinci Resolve DNxHR HQ* & *Apple ProRes 422* with uncompressed 16-bit PCM audio).
- [x] **Step-by-Step Manual Wizard** — Granular configuration for resolution limits, custom timecode cuts, dual-audio tracks, and subtitles.
- [x] **Network Acceleration Engine** — Multi-fragment streaming (`--concurrent-fragments`), automated retries, and YouTube anti-throttling mechanisms baked in.
- [x] **SponsorBlock & Chapter Splitting** — Cut sponsor segments on-the-fly or split long podcasts and concerts into track-by-track files.
- [x] **High-Res Poster Extractor** — Standalone mode to fetch original video artwork converted to lossless PNG, JPG, or WEBP.

---

## 🚀 Installation

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
| <kbd>F12</kbd> / <kbd>Alt</kbd>+<kbd>Shift</kbd>+<kbd>P</kbd> | **Global** | Toggle floating interactive terminal overlay |
| <kbd>←</kbd> <kbd>→</kbd> or <kbd>h</kbd> <kbd>l</kbd> | **Main Menu** | Switch horizontal category tabs |
| <kbd>↑</kbd> <kbd>↓</kbd> or <kbd>k</kbd> <kbd>j</kbd> | **Navigation** | Move cursor selection up / down |
| <kbd>Enter</kbd> | **Dialogs** | Confirm selection / Open sub-menu |
| <kbd>Esc</kbd> / <kbd>q</kbd> | **Runner** | Transfer active task to background queue |
| <kbd>F10</kbd> | **Runner** | Toggle between clean stage view and raw stdout log |
| <kbd>Ctrl</kbd>+<kbd>C</kbd> | **Runner** | Force kill current process |

---

## 💻 Floating Terminal Commands

Access via <kbd>F12</kbd> to control the engine directly:

```ansi
mediacli> help
  config [list | get <k> | set <k> <v>]  - Inspect and modify runtime parameters
  preset [list | delete <id>]            - Manage saved download profiles
  queue  [list | cancel <id>]            - Inspect/terminate background workers
  history [list | clear]                 - View operation log history
  cookies [list | add <dom> <k> <v>]     - Manage Netscape cookie file
  theme  <name>                          - Switch UI color palette on the fly
  doctor                                 - Diagnose local binary dependencies
  dl     <url>                           - Dispatch immediate background download
```

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
[ tcell TUI ]    [ Dynamic Parser ]    [ Background Queue ]
(Double-Buffer)   (Regex Stream)        (Goroutine Worker Pool)
   │                     │                       │
   └───────────────┬─────┴───────────────────────┘
                   ▼
     [ os/exec Stream Pipeline ]
                   │
         ┌─────────┴─────────┐
         ▼                   ▼
    { yt-dlp }          { FFmpeg }
```

> [!NOTE]
> MediaCLI is compiled with `CGO_ENABLED=0` to create a 100% statically linked binary without runtime glibc version constraints.

---

## 📄 License

Distributed under the **MIT License**. See [`LICENSE`](LICENSE) for more information.
