# Frontend

This folder holds the KoalaPull UI.

## Shape

The frontend is a single-page React app built with Vite.
It talks to the Go backend through Wails bindings and Wails events.

### Wails bindings

Generated files live in `wailsjs/go/main/`.
They expose the Go app methods used by the UI for metadata, downloads, settings, and version data.

### Wails events

Live backend updates stream into React through named events such as:

- `download-progress`
- `dependency-progress`

## Theme

Theme tokens live in `src/style.css` and `tailwind.config.js`.

- Surfaces and text colors use CSS variables.
- Accent color is teal, not purple.
- Fonts are bundled locally: Inter and JetBrains Mono.

Shared classes such as `btn-primary`, `select-dark`, and `input-dark` are built with `@apply`.

## Key Files

| File | Purpose |
|------|---------|
| `src/App.tsx` | Main app UI, state, and view logic |
| `src/style.css` | Global styles, theme vars, shared classes |
| `src/main.tsx` | React mount point |
| `tailwind.config.js` | Tailwind theme config |
| `vite.config.ts` | Vite config |
| `index.html` | App shell and CSP |

## Dev

```bash
cd frontend
npm install
npm run dev
```

That starts Vite only.
For the full app, run `wails dev` from repo root.

## Test and Build

```bash
npm run test
npm run build
```

`npm run build` emits `frontend/dist/`, which Wails embeds in the desktop build.
