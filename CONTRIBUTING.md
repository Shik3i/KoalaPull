# Contributing to KoalaPull

Thank you for your interest in contributing to KoalaPull! This document provides guidelines for setting up your local environment, building the application, and submitting contributions.

---

## 🛠️ Local Development Setup

To configure your development environment, ensure you have the following installed:

- **Go** 1.23+
- **Node.js** 18+
- **Wails CLI** v2.12+:
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```
- **macOS**: Xcode Command Line Tools
- **Linux**: Install development dependencies: `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `pkg-config`
- **Windows**: No additional system dependencies required.

Verify your installation by running:
```bash
wails version
```

---

## 📦 Getting Started

1.  **Clone the Repository:**
    ```bash
    git clone https://github.com/Shik3i/KoalaPull.git
    cd KoalaPull
    ```

2.  **Install Frontend Dependencies:**
    ```bash
    cd frontend && npm install && cd ..
    ```

3.  **Start in Development Mode:**
    ```bash
    wails dev
    ```
    This launches the application with hot-reloading for both Go code and frontend React components.

---

## 🏗️ Repository Architecture

-   `app.go`: Controls the Go backend, coordinating downloads, clipboard checks, version management, and local database history.
-   `main.go`: Defines the desktop app wrapper and mounts the Wails application instance.
-   `process_*.go`: Contains OS-specific process-spawning and helper integrations.
-   `frontend/`: The React, TypeScript, and Vite single-page application.
    -   `frontend/src/App.tsx`: Main user interface file.
    -   `frontend/src/style.css`: Theme variables and Tailwind declarations.
-   `scripts/verify.sh`: Repository validation script.
-   `build/`: Base icon graphics, plist setups, and compilation binaries.

---

## 🎨 Coding Style & Verification

### Backend (Go)
Keep Go code formatted using the official standards:
```bash
# Format all packages
gofmt -s -w .

# Run static analysis
go vet ./...
```

### Frontend (TypeScript / React)
Ensure TypeScript types evaluate successfully before submitting edits:
```bash
cd frontend
npx tsc --noEmit
```

---

## 🚦 Pre-Push Quality Gate

Before pushing any changes or raising a pull request, you **must** run the verification script to ensure the codebase remains green:

```bash
./scripts/verify.sh
```

This verification script automatically runs:
1.  **Vitest Suite:** `npm run test` inside the `frontend` folder.
2.  **Frontend Compilation check:** `npm run build` inside the `frontend` folder.
3.  **Go Test Suite:** `go test -count=1 ./...`
4.  **Go static checks:** `go vet ./...`
5.  **GitHub Action workflow validations.**

---

## 📥 Pull Request Guidelines

-   **Single Focus:** Keep each pull request centered on a single topic, bug fix, or feature addition.
-   **Add Tests:** If you are modifying download calculations, i18n behaviors, or other logic files, write/update corresponding tests.
-   **Update Documentation:** Keep markdown guides updated if you alter configuration names, build options, or runtime behavior.
-   **Verify Locally First:** Make sure the quality gate script (`./scripts/verify.sh`) succeeds locally before creating a pull request.
