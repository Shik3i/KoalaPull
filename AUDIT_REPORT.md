# KoalaPull Audit Remediation Report

Date: 2026-06-06

## Reverified Findings

### SSRF guard after preflight
**Severity:** HIGH  
**Domain:** Backend  
**Status:** Fixed by narrowing accepted download hosts to the app's supported public site allowlist before launching `yt-dlp`. The original risk existed because only the submitted hostname was resolved before `yt-dlp` handled later network behavior.

### Mutable dependency binary trust
**Severity:** HIGH
**Domain:** Infrastructure
**Status:** Partly mitigated. Managed downloads now enforce trusted hosts at initial request and redirect time, bounded reads, and checksums. macOS moved off the retired Evermeet URL and installs both `ffmpeg` and `ffprobe` from the same checked channel. Full upstream-compromise protection still requires signed metadata/TUF/Sigstore support from the chosen distributors.

### Advanced custom argument abuse
**Severity:** MEDIUM  
**Domain:** Backend  
**Status:** Fixed. Header and user-agent passthrough were removed, value-bearing options now have bounded validators, and tests cover rejected abuse values.

### Wails CSP too permissive
**Severity:** MEDIUM
**Domain:** Frontend
**Status:** Fixed for script and remote image execution surface. `script-src` no longer allows inline scripts, and `img-src` is limited to `self data:`. Inline styles remain allowed because the current UI uses React style attributes extensively.

### Remote thumbnail/favicons privacy surface
**Severity:** MEDIUM  
**Domain:** Full-Stack  
**Status:** Fixed. Backend no longer returns remote thumbnails to the UI, metadata/queue thumbnails render local imagery only, and supported-site badges no longer fetch Google favicon URLs.

### Browser termination too broad
**Severity:** LOW
**Domain:** Backend / UX
**Status:** Fixed at the backend boundary. `KillBrowser` now refuses automatic process termination for known browsers and tells the user to close the browser manually.

### Muted text contrast
**Severity:** LOW  
**Domain:** Frontend  
**Status:** Fixed. Dark-mode muted text was raised from `#6b7280` to `#9ca3af`.

## Regression Coverage

- Custom `yt-dlp` argument validation tests.
- Supported download-host allowlist tests.
- CSP regression test for inline script and remote image restrictions.
- Frontend regression test blocking remote thumbnail/favicon render paths.
- Existing dependency integrity and bounded extraction tests still pass.

## Verification

- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `npm run test` in `frontend/` passed.
- `npm run build` in `frontend/` passed.
- `npm audit --audit-level=moderate` in repo root passed.
- `npm audit --audit-level=moderate` in `frontend/` passed.
