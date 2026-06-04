# KoalaPull

KoalaPull is a native desktop download manager for [yt-dlp](https://github.com/yt-dlp/yt-dlp).
It downloads videos, audio, playlists, subtitles, and metadata from hundreds of sites with a desktop UI instead of terminal commands.

## Key Features

- Zero-config setup: automatically downloads and configures `yt-dlp` and `ffmpeg` on first run.
- Hardened dependency updates: downloads are size-limited, integrity-checked, archive-validated, and atomically replaced.
- Direct engine updates: update or re-install `yt-dlp` and `ffmpeg` from the Settings tab.
- Cross-platform UI: built with Go, React, and Wails for macOS, Windows, and Linux.
- Metadata preview: inspect thumbnails, uploader data, duration, and formats before downloading.
- Queue and presets: parallel downloads, presets, subtitle options, and history built in.
- Privacy-first: local-first workflow, no telemetry, no tracking, no external CDN requirement.

## Installation

Download the latest release from:

- [GitHub Releases](https://github.com/Shik3i/KoalaPull/releases)

Current packaged targets:

- macOS arm64
- macOS amd64
- Windows amd64
- Linux amd64

## Building From Source

Requirements:

- Go 1.26.4 or newer
- Node.js 22 or newer
- Wails CLI v2.12.0 or newer
- Linux only: `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `pkg-config`

Build and run locally:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
git clone https://github.com/Shik3i/KoalaPull.git
cd KoalaPull
wails dev
```

Production build:

```bash
wails build -clean -ldflags "-X main.AppVersion=$(git describe --tags --always --dirty)"
```

## Runtime Dependency Flow

KoalaPull stores downloaded engine binaries here:

- Linux: `~/.config/KoalaPull/bin/`
- macOS: `~/Library/Application Support/KoalaPull/bin/`
- Windows: `%APPDATA%/KoalaPull/bin/`

Downloaded tools:

- `yt-dlp`
- `ffmpeg`

### Hardened Download Flow

When KoalaPull downloads or updates `yt-dlp` and `ffmpeg`, it now uses this flow:

1. Download into temporary files with hard size limits.
2. Verify integrity before install:
   - `yt-dlp`: SHA-256 checksum verification.
   - `ffmpeg` on Windows/Linux: upstream SHA-256 checksum verification.
   - `ffmpeg` on macOS: detached signature verification against the embedded Evermeet signing key.
3. Validate archive member paths and extract only the expected binaries with bounded extraction.
4. Replace old binaries atomically so failed updates do not leave half-written executables behind.
5. On Windows, keep `ffmpeg.exe` and `ffprobe.exe` together.

Result:

- broken downloads are rejected
- oversized downloads are rejected
- archive traversal is blocked
- partial installs should not replace working binaries

## Verification Workflow

There is now one canonical verifier:

- `scripts/verify.mjs`

Launchers:

- Unix: `./scripts/verify.sh`
- Windows: `.\scripts\verify.bat`

The verifier runs:

1. `frontend`: `npm ci --include=optional`
2. `frontend`: `npm run test`
3. `frontend`: `npx tsc --noEmit`
4. `frontend`: `npm run build`
5. `frontend`: `npm audit --audit-level=moderate`
6. repository root: `npm ci --include=optional` for website tooling
7. `website`: `node --test`
8. Go tests: `go test -count=1 ./...`
9. Go race tests: `go test -race -count=1 ./...`
10. `go vet ./...`
11. `govulncheck`
12. `actionlint` for workflow validation

## Project Layout

```text
KoalaPull/
|- app.go
|- app_test.go
|- dependency_security.go
|- process_other.go
|- process_windows.go
|- process_output.go
|- replace_other.go
|- replace_windows.go
|- scripts/
|  |- verify.mjs
|  |- verify.sh
|  |- verify.bat
|- frontend/
|- website/
|- build/
|- docs/
`- wails.json
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
