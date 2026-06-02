# KoalaPull

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Shik3i/KoalaPull)](https://goreportcard.com/report/github.com/Shik3i/KoalaPull)
![Go Version](https://img.shields.io/badge/Go-1.23+-blue)
![Wails](https://img.shields.io/badge/Wails-v2.12.0-blue)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)

A clean, native desktop download manager wrapping **yt-dlp** — download videos, audio, and playlists from hundreds of sites with a simple GUI.

</div>

---

## Features

- **Dependency auto-setup** — yt-dlp and ffmpeg are downloaded and isolated to your app data directory on first launch (no system-wide install required). macOS binaries automatically have their quarantine attribute removed.
- **Cross-platform native UI** — macOS, Windows, and Linux via Wails (WKWebView / WebView2 / WebKitGTK). Consistent look and feel on every OS.
- **Metadata preview** — Paste a URL, fetch video title, uploader, thumbnail, duration, and available formats before downloading.
- **Format selection** — Choose resolution (sorted by quality), container (MP4 / MKV / MP3), and subtitle options (none, auto-generated, or all languages).
- **Playlist support** — Fast metadata via `--flat-playlist`; download all videos with your chosen format settings. Per-video progress displayed live.
- **Concurrent downloads** — Queue multiple downloads and run up to 10 in parallel (configurable per your system).
- **Download queue** — Add, cancel, and clear items. Real-time progress, speed, ETA, and playlist status streamed live from the Go backend.
- **Download history** — Auto-saved with timestamps, file sizes, average speed, and status. Filter by URL or title, delete individual entries, or clear all.
- **Theme support** — Toggle between dark and light themes. Preference is persisted across sessions.
- **Auto-paste URLs** — Optional setting that automatically detects YouTube URLs from your clipboard.
- **Update notifications** — Settings tab checks for new yt-dlp releases via the GitHub API (on-demand, no background polling). A badge appears on the Settings tab when an update is available.
- **Privacy-first** — All fonts are bundled locally. A restrictive Content Security Policy blocks unwanted network requests from the webview. No telemetry, no analytics, no external CDNs.
- **Keyboard shortcuts** — `Cmd/Ctrl+K` or `Cmd/Ctrl+L` focuses the URL input.

---

## Quick Start

### Download

Grab the latest release for your platform:

| Platform | File |
|----------|------|
| **macOS** (Intel) | `koalapull-darwin-amd64.dmg` |
| **macOS** (Apple Silicon) | `koalapull-darwin-arm64.dmg` |
| **Windows** | `koalapull-windows-amd64.exe` |
| **Linux** | `koalapull-linux-amd64.AppImage` |

> On first launch you'll see a **Setup** screen. Click **Download & Install** to fetch yt-dlp and ffmpeg — they're stored in your app config directory, not system-wide. After that, the main UI loads and you're ready to go.

### Build from Source

```bash
# Prerequisites: Go 1.23+, Node.js 18+, Wails CLI v2.12+
go install github.com/wailsapp/wails/v2/cmd/wails@latest

git clone https://github.com/Shik3i/KoalaPull.git
cd KoalaPull
cd frontend && npm install && cd ..
wails dev
```

---

## Usage

### Your first download

1. **Paste a URL** — from YouTube, Vimeo, Twitch, or any site yt-dlp supports.
2. **Fetch metadata** — click "Fetch Metadata" or hit `Enter`. The app loads the video title, thumbnail, duration, and available formats.
3. **Select options** — choose resolution, container (MP4/MKV/MP3), and subtitles.
4. **Add to queue** — click "Add to Queue". The download starts immediately if fewer than your configured concurrency limit are running.
5. **Monitor progress** — each item shows real-time progress, speed, and ETA. Cancel or clear items as needed.

### Settings

| Setting | Range | Default | Description |
|---------|-------|---------|-------------|
| Output directory | any folder | `~/Downloads/KoalaPull` | Where completed files are saved |
| Theme | dark / light | dark | App colour scheme |
| Max parallel downloads | 1 – 10 | 3 | How many downloads run simultaneously |
| Auto-paste URL | on / off | off | Automatically detect YouTube URLs in your clipboard |

All settings are persisted to `settings.json` in your app config directory.

---

## Dependencies

### Runtime

| Dependency | Purpose | Source |
|---|---|---|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | Download engine (handles all sites) | Downloaded on first launch |
| [ffmpeg](https://ffmpeg.org) | Video/audio processing and merging | Downloaded on first launch |

> Both binaries are downloaded to your app data directory (`~/.config/KoalaPull/bin/` on Linux, `~/Library/Application Support/KoalaPull/bin/` on macOS, `%APPDATA%/KoalaPull/bin/` on Windows). No system-wide installation needed. Updates can be triggered from the Settings tab.

### Go (backend)

| Module | Version | Purpose |
|---|---|---|
| `github.com/wailsapp/wails/v2` | `v2.12.0` | Desktop app framework (native webview + Go IPC bridge) |

Wails v2 is the **only direct Go dependency** — all other entries in `go.mod` are transitive dependencies of Wails itself.

### Frontend

| Package | Version | Type |
|---|---|---|
| `react` | `^18.2.0` | Runtime dependency |
| `react-dom` | `^18.2.0` | Runtime dependency |
| `vite` | `^3.0.7` | Build tool |
| `tailwindcss` | `^3.4.19` | Utility-first CSS framework |
| `typescript` | `^4.6.4` | Type safety |

### Fonts

Inter (300–700) and JetBrains Mono (400, 500) are bundled as TTF files in `frontend/public/fonts/`. **No external font CDN** — zero network requests for typography.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.23, Wails v2.12 |
| Frontend | React 18, TypeScript, Vite 3 |
| Styling | Tailwind CSS 3, CSS custom properties (dark/light themes) |
| Download engine | yt-dlp + ffmpeg |

---

## Project Structure

```
KoalaPull/
├── app.go                 # Go backend — dependencies, metadata, downloads, settings, history
├── main.go                # Wails app entry point
├── frontend/
│   ├── src/
│   │   ├── App.tsx        # Main React component (sidebar + tabs)
│   │   ├── style.css      # Tailwind directives + theme variables + local font faces
│   │   └── main.tsx       # React mount point
│   ├── public/fonts/      # Bundled local fonts (Inter, JetBrains Mono)
│   └── index.html         # Entry point with Content Security Policy
├── build/                 # Build assets (icons, plists, manifests)
├── wails.json             # Wails project configuration
├── .github/workflows/     # CI/CD (GoReleaser for cross-platform builds)
├── go.mod                 # Go module definition
└── frontend/package.json  # Frontend dependencies
```

---

## Development

### Prerequisites

- [Go](https://go.dev/dl/) 1.23+
- [Node.js](https://nodejs.org/) 18+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2.12+
- **Linux only:** `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `pkg-config`

### Commands

```bash
# Development (hot-reload for Go + React)
wails dev

# Production build
wails build -clean -ldflags "-X main.AppVersion=$(git describe --tags --always --dirty)"
```

The compiled binary is written to `build/bin/`. Release builds inject the git tag as the app version via `-ldflags`.

---

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and coding guidelines.

**Code of Conduct:** Please note that this project adheres to a [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold its terms.

---

## License

[MIT](LICENSE) © 2026 Timo
