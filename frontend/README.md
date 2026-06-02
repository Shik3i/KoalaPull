# Frontend — React + TypeScript + Vite + Tailwind CSS

This directory contains the KoalaPull user interface.

## Architecture

The UI is a single-page React application bootstrapped by Vite. It communicates with the Go backend through two Wails mechanisms:

1. **Wails Bindings** (sync) — Direct async function calls to Go methods, e.g.:
   - `FetchMetadata(url)` → returns `VideoMetadata`
   - `StartDownload(url, format, dir, container, subtitle)` → returns `downloadId`
   - `GetSettings()` / `UpdateSettings()` → read/write settings

2. **Wails Events** (async) — Real-time streaming from Go to React:
   - `download-progress` — percent, speed, ETA, playlist status
   - `dependency-progress` — yt-dlp/ffmpeg download progress

Auto-generated binding stubs live in `wailsjs/go/main/App.js` and `App.d.ts`. They are maintained manually when the Wails `generate module` command is unavailable.

## Tailwind Configuration

The theme is defined in `tailwind.config.js`:

- **Surface colors**: `#111111` (base), `#1a1a1a` (light), `#252525` (lighter), `#2a2a2a` (border)
- **Accent**: `#00d4aa` (cyan/turquoise)
- **Fonts**: Inter (UI), JetBrains Mono (code)

Custom component classes (`btn-primary`, `select-dark`, `input-dark`) are composed with `@apply` in `style.css`.

## Key Files

| File | Purpose |
|------|---------|
| `src/App.tsx` | Main application component (state, effects, layout) |
| `src/style.css` | Tailwind directives + reusable component classes |
| `tailwind.config.js` | Dark theme colors, fonts, content paths |
| `vite.config.ts` | Vite configuration (React plugin, dev server) |
| `index.html` | Vite entry point (mounts `<div id="app">`) |

## Development

```bash
cd frontend
npm install
npm run dev       # Standalone Vite dev server (browser only, no Go backend)
```

For full app with Go backend, run `wails dev` from the project root.

## Build

```bash
npm run build     # tsc + vite build
```

The output goes to `frontend/dist/` and is embedded by Wails during `wails build`.
