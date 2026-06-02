# Contributing to KoalaPull

Thanks for helping with KoalaPull.
This guide keeps setup, style, and release steps in one place.

## Setup

### Prerequisites

- **Go** 1.23+
- **Node.js** 18+
- **Wails CLI** v2.12+:
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```
- **macOS**: Xcode Command Line Tools
- **Linux**: `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `pkg-config`
- **Windows**: no extra setup

### Check

```bash
wails version
```

## Repo Shape

```text
KoalaPull/
├── app.go              # Go backend: downloads, settings, history, updates
├── main.go             # Wails app entry point
├── frontend/
│   ├── src/App.tsx     # React UI
│   ├── src/style.css   # Theme vars and shared classes
│   └── wailsjs/        # Generated Wails bindings
├── build/              # Native build assets
└── wails.json          # Wails config
```

## Core Choices

- `yt-dlp` and `ffmpeg` live in the app config dir, not in the install folder.
- Download progress comes from Wails events.
- Settings save as JSON in the app config dir.
- Parallel downloads are capped in Go.

## Local Run

1. Clone repo.
2. Install frontend deps:
   ```bash
   cd frontend && npm install && cd ..
   ```
3. Start dev mode:
   ```bash
   wails dev
   ```

## Build

```bash
wails build -clean
```

Output lands in `build/bin/`.

## Style

- Go: `gofmt`, `go vet ./...`
- Frontend: `npx tsc --noEmit`
- UI docs: keep the dark/minimal theme language aligned with the app

## Before Push

Run the repo gate:

```bash
./scripts/verify.sh
```

It runs:

- `go test -count=1 ./...`
- `go vet ./...`
- `npm run test`
- `npm run build`

## PRs

- One topic per PR.
- Add tests when behavior changes.
- Update docs when user-facing behavior changes.
