# Contributing to KoalaPull

Thank you for your interest in contributing! This guide will help you set up the project locally and submit changes.

## Development Environment

### Prerequisites

- **Go** 1.23+ — [download](https://go.dev/dl/)
- **Node.js** 18+ — [download](https://nodejs.org/)
- **Wails CLI** v2.12+
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```
- **Platform-specific tools**:
  - **macOS**: Xcode Command Line Tools (`xcode-select --install`)
  - **Linux**: WebKit2GTK + GTK3 dev packages
    ```bash
    sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config
    ```
  - **Windows**: No additional tools needed (WebView2 is bundled)

### Verify Installation

```bash
wails version
# Expected: v2.12.0
```

## Project Architecture

```
KoalaPull/
├── app.go              # Go backend — deps, metadata, downloads, settings
├── main.go             # Wails app entry point
├── frontend/
│   ├── src/App.tsx     # React UI (components, state, event handlers)
│   ├── src/style.css   # Tailwind + custom component classes
│   └── wailsjs/        # Auto-generated Wails bindings (manually synced)
├── build/              # Platform build assets (icons, manifests, plists)
└── wails.json          # Wails project configuration
```

### Key Design Decisions

- **Binary isolation**: `yt-dlp` and `ffmpeg` are stored in `os.UserConfigDir()/KoalaPull/bin` — never in the installation directory.
- **Concurrency**: A Go semaphore (buffered channel, cap 3) limits parallel downloads. Context cancellation prevents zombie processes.
- **Progress**: Real-time progress is streamed from Go to React via Wails Events (`download-progress`).
- **Settings**: Persisted as JSON in `os.UserConfigDir()/KoalaPull/settings.json`.

## Setting Up for Development

1. Clone the repository:
   ```bash
   git clone https://github.com/Shik3i/KoalaPull.git
   cd KoalaPull
   ```

2. Install frontend dependencies:
   ```bash
   cd frontend && npm install && cd ..
   ```

3. Run in development mode (hot-reload):
   ```bash
   wails dev
   ```
   This starts a Vite dev server for the React frontend and a Wails dev server for the Go backend. The app window opens automatically.

4. To build a production binary:
   ```bash
   wails build -clean
   ```
   Output is placed in `build/bin/`.

## Code Style

- **Go**: Follow `gofmt` conventions. Run `go vet ./...` before committing.
- **React/TypeScript**: The project uses the `tsc` type checker. Run `npx tsc --noEmit` in `frontend/` before committing.
- **CSS**: Use Tailwind utility classes. Custom component classes are defined in `style.css` using `@apply`. Follow the existing dark minimalist theme with the `#00d4aa` accent.

## Making Changes

1. Create a feature branch:
   ```bash
   git checkout -b feat/my-feature
   ```

2. Make your changes and test them with `wails dev`.

3. Run the full verification gate before every push:
   ```bash
   ./scripts/verify.sh
   ```
   This is mandatory for normal pushes and release tags. The script runs:
   - `go test -count=1 ./...`
   - `go vet ./...`
   - `npm run test`
   - `npm run build`

4. Commit with a conventional commit message:
   ```
   feat: add playlist download support
   fix: prevent crash on empty URL
   chore: update dependencies
   ```

5. Push and open a Pull Request:
   ```bash
   git push origin feat/my-feature
   ```

## Pull Request Guidelines

- Keep PRs focused on a single concern.
- Update or add tests if applicable.
- Update documentation if the API or behavior changes.
- Reference any related issues in the PR description.

## Questions?

Open a [Discussion](https://github.com/Shik3i/KoalaPull/discussions) or file an [Issue](https://github.com/Shik3i/KoalaPull/issues).
