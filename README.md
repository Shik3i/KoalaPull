<p align="center">
  <img src="assets/Icon.png" alt="KoalaPull" width="128">
</p>

<h1 align="center">KoalaPull</h1>

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/Shik3i/KoalaPull?logo=github)](https://github.com/Shik3i/KoalaPull/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/Shik3i/KoalaPull)](https://goreportcard.com/report/github.com/Shik3i/KoalaPull)
![Go Version](https://img.shields.io/badge/Go-1.23+-blue)
![Wails](https://img.shields.io/badge/Wails-v2.12.0-blue)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)

A clean, native desktop download manager for **yt-dlp**.
Download videos, audio, playlists, subtitles, and metadata from hundreds of supported sites with a simple GUI.

</div>

---

## Overview

KoalaPull wraps `yt-dlp` in a native desktop app built with Go and Wails.
It is made for people who want the power of `yt-dlp` without living in the terminal.

## Features

- **Dependency auto-setup** - `yt-dlp` and `ffmpeg` are downloaded into your app data directory on first launch.
- **Dependency updates** - re-download or update `yt-dlp` / `ffmpeg` from the Settings tab anytime.
- **Cross-platform native UI** - runs on macOS, Windows, and Linux.
- **Metadata preview** - fetch title, uploader, thumbnail, duration, and formats before downloading.
- **Download presets** - Best Quality, Compatible, Audio Only, or Custom with full control.
- **Format selection** - pick resolution, container (MP4/MKV/MP3), and subtitle mode.
- **Playlist support** - download full playlists with one flow.
- **Concurrent downloads** - queue multiple jobs and run up to 10 in parallel.
- **Download queue** - add, cancel, clear completed, and clear all with live progress, speed, ETA, and total ETA.
- **Download history** - keep track of past downloads with search/filter by title or URL.
- **Open output folder** - jump to the file location straight from a completed download.
- **Theme support** - dark and light themes with saved preference.
- **Auto-paste URLs** - optional clipboard detection for supported video links.
- **Internationalization** - English, German, and French UI with automatic language detection.
- **Error boundary** - graceful crash recovery with a Retry button.
- **Update notifications** - check for new `yt-dlp` releases from the Settings tab.
- **Version info** - view installed KoalaPull, yt-dlp, and ffmpeg versions in Settings.
- **Help tab** - quick app guide, 24 curated supported sites, and a link to the full `yt-dlp` list.
- **Privacy-first** - bundled fonts, strict CSP, no telemetry, no analytics, no external CDNs.
- **Keyboard shortcuts** - quick focus on the URL input with `Cmd/Ctrl+K` or `Cmd/Ctrl+L`.

## Quick Start

### Download

Get the latest release for your platform from the [GitHub Releases page](https://github.com/Shik3i/KoalaPull/releases).

| Platform | File |
|----------|------|
| **macOS** (Intel) | `koalapull-darwin-amd64.dmg` |
| **macOS** (Apple Silicon) | `koalapull-darwin-arm64.dmg` |
| **Windows** | `koalapull-windows-amd64.exe` |
| **Linux** | `koalapull-linux-amd64.AppImage` |

### Note for macOS Users (Unidentified Developer)

KoalaPull is unsigned. macOS Gatekeeper may block it on first launch.

**Method 1: The Right-Click Shortcut (Recommended)**

1. Do not double-click the app.
2. Right-click, or `Control`-click, the KoalaPull app.
3. Choose **Open** from the menu.
4. You will see a warning dialog, but this time it includes an **Open** button. Click it.
5. KoalaPull will open normally from then on.

**Method 2: System Settings**

1. If you already double-clicked the app and saw the "cannot be opened" message, click **OK**.
2. Open **System Settings** > **Privacy & Security**.
3. Scroll down to the **Security** section.
4. Find the message that says KoalaPull was blocked.
5. Click **Open Anyway** and confirm with your Mac password.

On first launch, KoalaPull will show a setup screen.
Click **Download & Install** to fetch `yt-dlp` and `ffmpeg`.

### Build from Source

```bash
# Prerequisites: Go 1.23+, Node.js 18+, Wails CLI v2.12+
go install github.com/wailsapp/wails/v2/cmd/wails@latest

git clone https://github.com/Shik3i/KoalaPull.git
cd KoalaPull
cd frontend && npm install && cd ..
wails dev
```

## Usage

1. Paste a URL from YouTube, Vimeo, Twitch, or any other supported site.
2. Fetch metadata to inspect title, thumbnail, duration, and available formats.
3. Choose your format, container, and subtitle options.
4. Add the job to the queue.
5. Watch progress, speed, ETA, and playlist status in real time.
6. Open the Help tab for a quick guide and supported sites overview.

## Settings

| Setting | Range | Default | Description |
|---------|-------|---------|-------------|
| Output directory | any folder | `~/Downloads/KoalaPull` | Where completed files are saved |
| Theme | dark / light | dark | App color scheme |
| Language | auto / en / de / fr | auto | UI language |
| Max parallel downloads | 1 - 10 | 3 | How many downloads run at once |
| Auto-paste URL | on / off | off | Detect supported URLs from your clipboard |

All settings are stored in `settings.json` inside the app config directory.

## Dependencies

### Runtime

| Dependency | Purpose | Source |
|---|---|---|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | Download engine | Downloaded on first launch |
| [ffmpeg](https://ffmpeg.org) | Audio and video processing | Downloaded on first launch |

Both binaries are stored in the app data directory:

- Linux: `~/.config/KoalaPull/bin/`
- macOS: `~/Library/Application Support/KoalaPull/bin/`
- Windows: `%APPDATA%/KoalaPull/bin/`

### Go backend

| Module | Version | Purpose |
|---|---|---|
| `github.com/wailsapp/wails/v2` | `v2.12.0` | Desktop app framework |

### Frontend

| Package | Version | Type |
|---|---|---|
| `react` | `^18.2.0` | Runtime dependency |
| `react-dom` | `^18.2.0` | Runtime dependency |
| `vite` | `^3.0.7` | Build tool |
| `tailwindcss` | `^3.4.19` | Styling |
| `typescript` | `^4.6.4` | Type safety |

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.23, Wails v2.12 |
| Frontend | React 18, TypeScript, Vite 3 |
| Styling | Tailwind CSS 3, CSS custom properties |
| i18n | Custom locale system (en, de, fr) |
| Download engine | `yt-dlp` + `ffmpeg` |

## Project Structure

```text
KoalaPull/
├── app.go                 # Go backend: metadata, downloads, settings, history
├── app_test.go            # Backend tests
├── main.go                # Wails app entry point
├── process_other.go       # Platform-specific helpers (Linux/macOS)
├── process_windows.go     # Platform-specific helpers (Windows)
├── frontend/
│   ├── src/
│   │   ├── App.tsx        # Main React component
│   │   ├── main.tsx       # React mount point
│   │   ├── style.css      # Tailwind directives + theme variables
│   │   ├── vite-env.d.ts  # Vite type declarations
│   │   ├── assets/        # Local images and fonts
│   │   │   ├── images/
│   │   │   └── fonts/
│   │   ├── lib/           # Utility modules
│   │   │   ├── downloadMetrics.ts
│   │   │   └── i18n.ts
│   │   └── locales/       # Translation files (en, de, fr)
│   ├── public/fonts/      # Bundled local fonts
│   ├── index.html         # Entry point with Content Security Policy
│   └── package.json       # Frontend dependencies
├── assets/                # App icon sources
│   ├── Icon.png
│   └── Icon.af
├── build/                 # Build assets and platform packages
│   ├── appicon.png
│   ├── darwin/
│   ├── windows/
│   └── bin/
├── scripts/
│   └── verify.sh          # Pre-push quality gate
├── test/
│   └── quality_gates_test.go
├── docs/
│   └── superpowers/       # Design docs and plans
├── trayicons/             # System tray icon
├── website/               # Project website
├── wails.json             # Wails project configuration
├── .github/workflows/     # CI/CD pipelines
├── Makefile               # Dev convenience targets
├── go.mod                 # Go module definition
└── frontend/package.json  # Frontend dependencies
```

## Development

### Prerequisites

- [Go](https://go.dev/dl/) 1.23+
- [Node.js](https://nodejs.org/) 18+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2.12+
- Linux only: `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `pkg-config`

### Commands

```bash
# Development with hot reload
wails dev

# Production build
wails build -clean -ldflags "-X main.AppVersion=$(git describe --tags --always --dirty)"
```

The compiled binary is written to `build/bin/`.

## Contributing

Contributions are welcome.
See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and coding guidelines.

## License

[MIT](LICENSE) © 2026 Timo

## Responsible Use

KoalaPull is provided under the MIT License and comes with no warranty.
Use it only in ways that follow your local laws and the terms of the sites you access.
The maintainers are not responsible for misuse of the software.
