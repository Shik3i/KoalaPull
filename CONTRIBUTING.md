# Contributing to KoalaPull

## Local Setup

Install:

- Go 1.26.4+
- Node.js 22+
- Wails CLI v2.12.0+
- macOS: Xcode Command Line Tools
- Linux: `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `pkg-config`
- Windows: no extra system dependency for normal builds

Optional:

- Windows local race detector: install a C compiler such as GCC

Install Wails:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

## Getting Started

```bash
git clone https://github.com/Shik3i/KoalaPull.git
cd KoalaPull
cd frontend && npm ci --include=optional && cd ..
npm ci --include=optional
wails dev
```

Why two `npm ci` calls:

- `frontend/` lockfile covers the desktop UI
- root lockfile covers website build/test tooling such as `sharp` and `esbuild`

## Repository Notes

- `app.go`: main Go backend flow
- `dependency_security.go`: hardened dependency download and extraction logic
- `process_*.go`: OS-specific process handling
- `frontend/`: React + TypeScript desktop UI
- `website/`: static website source and tests
- `scripts/verify.mjs`: canonical repository verifier
- `scripts/verify.sh`: Unix launcher
- `scripts/verify.bat`: Windows launcher

## Verification

Run before push:

Unix:

```bash
./scripts/verify.sh
```

Windows:

```powershell
.\scripts\verify.bat
```

The verifier runs:

1. frontend clean install
2. frontend tests
3. frontend type-check
4. frontend build
5. frontend audit
6. root clean install for website tooling
7. website tests
8. Go tests
9. Go race tests
10. `go vet`
11. `govulncheck`
12. `actionlint`

## Pull Requests

- Keep the PR focused.
- Update tests when logic changes.
- Update docs when workflow, configuration, or runtime behavior changes.
- Run the verifier locally before opening the PR.
