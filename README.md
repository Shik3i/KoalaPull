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

**A clean, native desktop download manager for [yt-dlp](https://github.com/yt-dlp/yt-dlp).**  
Download videos, audio, playlists, subtitles, and metadata from hundreds of sites with a modern desktop UI.

</div>

---

## 🌟 Key Features

*   **📦 Zero-Config Auto Setup:** Automatically downloads and configures the latest `yt-dlp` and `ffmpeg` binaries in your application data folder on first run.
*   **⚙️ Direct Engines Update:** Re-download, verify, or update dependencies directly from the **Settings** tab.
*   **🖥️ Cross-Platform Desktop UI:** Built using Go, React, and Wails. Feels like a native app on macOS, Windows, and Linux.
*   **🔍 Rich Metadata Preview:** Inspect thumbnails, uploader details, exact duration, and all available stream qualities before downloading.
*   **⚡ Advanced Queue & Speed:** Queue multiple downloads, configure up to 10 parallel downloads, and see detailed metrics (current speed, queue status, active ETA, and overall ETA).
*   **🎼 Flexible Presets & Subtitles:** Pick Best Quality, Compatible (MP4), Audio Only (MP3), or supply custom options. Bundle or burn subtitles with ease.
*   **📂 Download History & Folder Jump:** Easily search, filter, and track previous downloads, and open the destination folder directly.
*   **🌍 Multi-lingual UI:** English, German, and French translations with automatic locale detection.
*   **🔒 Privacy-First:** Direct backend connections, bundled local fonts, strict Content Security Policy, no cookies, no tracking, and no external CDNs.

---

## 🚀 Installation & Setup

### Download Precompiled Release

Get the latest build for your desktop operating system from the [GitHub Releases Page](https://github.com/Shik3i/KoalaPull/releases).

| Operating System | Package/Binary | Architecture |
|---|---|---|
| 🍎 **macOS** | `koalapull-darwin-arm64.dmg` | Apple Silicon (M1/M2/M3/M4) |
| 🍎 **macOS** | `koalapull-darwin-amd64.dmg` | Intel-based Macs |
| 🪟 **Windows** | `koalapull-windows-amd64.exe` | Standard 64-bit Windows |
| 🐧 **Linux** | `koalapull-linux-amd64.AppImage` | Standard 64-bit Linux distributions |

> [!IMPORTANT]
> **Important Note for macOS Users (Unidentified Developer)**
> 
> Because KoalaPull is unsigned, macOS Gatekeeper may block it on the first launch.
> 
> *   **Method 1 (Recommended):** Right-click (or `Ctrl`+click) the KoalaPull application icon, select **Open** from the context menu, and click the **Open** confirmation button.
> *   **Method 2:** Double-click the application icon, dismiss the alert, navigate to **System Settings** > **Privacy & Security** > **Security**, and click **Open Anyway**.

---

## 🔨 Building from Source

To build KoalaPull yourself, ensure you have the following prerequisites installed:
*   [Go](https://go.dev/) 1.23 or newer
*   [Node.js](https://nodejs.org/) 18 or newer
*   [Wails CLI](https://wails.io/docs/gettingstarted/installation) (`v2.12.0` or newer)
*   *Linux only:* Dev library packages (`libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `pkg-config`)

```bash
# Install Wails CLI globally if not already present
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clone and navigate to repository
git clone https://github.com/Shik3i/KoalaPull.git
cd KoalaPull

# Build and run with hot-reload enabled for developer environments
wails dev
```

To compile production-optimized standalone binaries:
```bash
wails build -clean -ldflags "-X main.AppVersion=$(git describe --tags --always --dirty)"
```
Output binaries will be generated inside the `build/bin/` folder.

---

## 🛠️ Configuration Settings

Customize settings from the UI interface. These configuration values are stored locally in a `settings.json` file.

| Configuration | Options | Default Value | Description |
|---|---|---|---|
| **Output Directory** | Any writeable path | `~/Downloads/KoalaPull` | Location for storing all downloaded assets. |
| **Theme Mode** | `dark` / `light` | `dark` | Visual theme interface. |
| **Interface Language** | `auto` / `en` / `de` / `fr` | `auto` | Preferred language of the interface elements. |
| **Parallel Downloads** | `1` to `10` | `3` | Maximum concurrent active download workers. |
| **Auto-paste Clipboard** | `on` / `off` | `off` | Autodetect and fetch supported media URLs from local clipboard. |

---

## 📦 Runtime Dependencies

KoalaPull executes self-contained binary wrappers. The local executables are downloaded and saved to:
*   **Linux:** `~/.config/KoalaPull/bin/`
*   **macOS:** `~/Library/Application Support/KoalaPull/bin/`
*   **Windows:** `%APPDATA%/KoalaPull/bin/`

| Dependency | Purpose | Resource Origin |
|---|---|---|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | Media extraction engine | Downloaded on first setup or Settings page updates |
| [ffmpeg](https://ffmpeg.org) | Video and audio transcoding | Downloaded on first setup or Settings page updates |

---

## 💻 Tech Stack

- **Backend:** Go 1.23, [Wails v2](https://wails.io/) desktop bindings
- **Frontend UI:** React 18, TypeScript, Vite
- **Styling:** Tailwind CSS 3, custom CSS theme variables
- **Localization:** Custom JSON dictionary system (`en`, `de`, `fr`)
- **Download Engines:** Wrapper layer for `yt-dlp` and `ffmpeg`

---

## 📂 Project Architecture

```text
KoalaPull/
├── app.go                 # App controllers (metadata fetching, settings, history)
├── app_test.go            # Go tests
├── main.go                # Wails framework entrypoint
├── process_other.go       # Non-Windows platform helper integrations
├── process_windows.go     # Windows platform helper integrations
├── go.mod                 # Go module definition
├── wails.json             # Wails layout properties
├── Makefile               # Shortcuts for common tasks
├── assets/                # App artwork assets
├── trayicons/             # System tray icon assets
├── scripts/               # Quality validation pipelines
│   └── verify.sh          # CI and commit gate verifications
├── docs/                  # Technical documentation and specs
├── website/               # Static landing website code
├── build/                 # Bundled files & distribution platform assets
└── frontend/              # Frontend application project folder
    ├── index.html         # Main HTML framework and security policy
    ├── package.json       # Node package structure
    └── src/
        ├── main.tsx       # React initialization
        ├── App.tsx        # Core UI views
        ├── style.css      # Core style templates and theme definitions
        ├── locales/       # JSON localization dictionaries
        └── lib/           # UI helpers (i18n, download calculation metrics)
```

---

## 🤝 Contributing

Contributions are highly encouraged. Please refer to [CONTRIBUTING.md](CONTRIBUTING.md) to learn how to set up the development environment, execute tests, and follow quality validation requirements.

---

## ⚖️ License & Disclaimers

This project is licensed under the [MIT License](LICENSE).

> [!WARNING]
> KoalaPull is designed to run as a wrapper layer around `yt-dlp`. It is provided "as is" without warranty of any kind. You are responsible for ensuring that your usage complies with local regulations, copyrights, and the terms of service of the websites from which you download content.
