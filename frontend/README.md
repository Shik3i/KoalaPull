# KoalaPull Frontend App

This folder holds the UI codebase for KoalaPull, implemented as a Single Page Application (SPA) using React, TypeScript, and Tailwind CSS.

---

## 🔗 Desktop Integration Architecture

The frontend interfaces with the Go application through the Wails framework.

### 1. Wails Bindings
Wails automatically translates Go methods on the App struct into client-side JavaScript promises. During builds or development runs, these bindings are written to `wailsjs/go/main/App.js` and can be imported directly into React:
```typescript
import { FetchMetadata, AddDownload } from '../wailsjs/go/main/App';
```

### 2. Wails Events
Real-time progress updates are pushed asynchronously from Go. The frontend subscribes to these event channels:
-   `download-progress`: Emits active download status, current transfer speed, and completed percentages.
-   `dependency-progress`: Emits updates during setup/installation of the `yt-dlp` or `ffmpeg` binaries.

---

## 🎨 Theme & Visual Layout

Theme styles are declared in `src/style.css` and referenced in `tailwind.config.js`.

-   **Color Palette:** The dominant theme uses a custom dark slate appearance accented with a teal hue (not purple).
-   **Typography:** System fonts are bundled locally (`public/fonts/`) to respect user privacy and avoid third-party resource loading:
    -   `Inter`: Applied for user interface copy and tables.
    -   `JetBrains Mono`: Applied for download metrics, command values, and technical stats.
-   **Tailwind Utilities:** Reusable elements like forms and buttons are declared using `@apply` utility definitions inside `src/style.css` (e.g. `.btn-primary`, `.input-dark`).

---

## 📁 Key File Structure

| Path | Purpose |
|---|---|
| `src/main.tsx` | React SPA mounting point. |
| `src/App.tsx` | Main application component containing all tabs, sidebar navigation, download state, and layout wrappers. |
| `src/style.css` | Application styles, theme variables, and custom Tailwind utility structures. |
| `src/lib/` | Frontend utility library (including i18n detection and download metric calculations). |
| `src/locales/` | Language files (`en.json`, `de.json`, `fr.json`). |
| `index.html` | Entry point defining strict Content Security Policies (CSP). |

---

## ⚙️ Development Commands

Before running commands, change directory to the frontend folder:
```bash
cd frontend
```

### 1. Install Dependencies
```bash
npm install
```

### 2. Development Dev Server
```bash
npm run dev
```
> [!NOTE]
> Running `npm run dev` only starts the Vite frontend dev server in a web browser. Wails-specific backend bindings will not resolve. To run the full application with the desktop shell and the Go backend, run `wails dev` from the repository root instead.

### 3. Run Tests
```bash
npm run test
```
Executes the Vitest suite validating formatting utilities, progress converters, and translation keys.

### 4. Build Production Bundle
```bash
npm run build
```
Compiles and bundles the application into `dist/`. Wails embeds this directory when packaging the final desktop binaries.
