# KoalaPull

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
![Go Version](https://img.shields.io/badge/Go-1.23+-blue)
![Wails](https://img.shields.io/badge/Wails-v2.12.0-blue)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)

</div>

A clean, minimalist GUI download manager wrapping **yt-dlp**. Download videos, audio, and playlists from hundreds of sites with a native desktop interface.

![Screenshot placeholder](docs/screenshot.png)

## Features

- **On-demand dependency setup** — yt-dlp and ffmpeg are downloaded and isolated to your app data directory on first launch after clicking "Download &amp; Install".
- **Cross-platform** — macOS, Windows, and Linux builds with a consistent native look using Wails (WebView2 / WKWebView / WebKitGTK).
- **Rich metadata preview** — Fetch video title, uploader, thumbnail, and available formats before downloading.
- **Format selection** — Choose resolution, container (MP4/MKV/MP3), and subtitle options.
- **Playlist support** — Fetch playlist metadata with `--flat-playlist` speed; apply format settings to all videos.
- **Concurrency limiting** — A Go semaphore caps parallel downloads at 3 to prevent saturating your CPU or network.
- **Download queue** — Add multiple items, cancel individual downloads, and clear completed items.
- **Real-time progress** — Stream progress, speed, ETA, and per-video playlist status live from the Go backend via Wails Events.
- **Dark mode UI** — High-contrast dark theme with cyan/turquoise accent, minimalist flat vector design.
- **Persistent settings** — Default output directory is stored and remembered across sessions.

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/Shik3i/KoalaPull/releases) tab.

| Platform | File |
|----------|------|
| **macOS** (Intel) | `koalapull-darwin-amd64.dmg` |
| **macOS** (Apple Silicon) | `koalapull-darwin-arm64.dmg` |
| **Windows** | `koalapull-windows-amd64.exe` |
| **Linux** | `koalapull-linux-amd64.AppImage` |

### Build from Source

See [Development Setup](#development-setup) below.

## Development Setup

### Prerequisites

- [Go](https://go.dev/dl/) 1.23+
- [Node.js](https://nodejs.org/) 18+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2.12+
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

- **Linux only**:
  ```bash
  sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config
  ```

### Quick Start

```bash
git clone https://github.com/Shik3i/KoalaPull.git
cd KoalaPull
cd frontend && npm install && cd ..
wails dev
```

This opens the app in a native window with hot-reload enabled for both Go and React code.

### Production Build

```bash
wails build -clean
```

The compiled binary is written to `build/bin/`.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.23 + Wails v2.12 |
| Frontend | React 18 + TypeScript + Vite 3 |
| Styling | Tailwind CSS 3 (custom dark theme) |
| Download Engine | yt-dlp + ffmpeg |

## Project Structure

```
KoalaPull/
├── app.go                 # Go backend (deps, metadata, download, settings)
├── main.go                # Wails app entry point
├── frontend/
│   ├── src/App.tsx        # Main React component
│   ├── src/style.css      # Tailwind directives + custom classes
│   └── wailsjs/           # Wails Go bindings (TypeScript + JS)
├── build/                 # Build assets (icons, plists, manifests)
├── wails.json             # Wails project config
└── .github/workflows/     # CI/CD workflows
```

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and coding guidelines.

## License

[MIT](LICENSE) © 2026 Timo
