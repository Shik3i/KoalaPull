# KoalaPull Audit Remediation Report

## Fixed Real Findings

### Red - Local/Private Network URL Exposure
**Severity:** HIGH  
**Domain:** Backend  
**Location:** `app.go:FetchMetadata`, `app.go:StartDownloadWithPreset`, `app.go:isAllowedDownloadURL`

**Status:** Fixed with hostname resolution checks before launching `yt-dlp`, existing literal IP blocking, and thumbnail URL sanitization.  
**Residual:** Redirects and extractor-discovered media URLs are still ultimately handled inside `yt-dlp`; a full network sandbox/proxy would be the stronger long-term control.

**Proof of Fix:**
```go
if err := validateDownloadURLForLaunch(a.appContext(), url); err != nil {
    return nil, err
}
```

### Red - Unsafe File Launch Through PlayFile
**Severity:** HIGH  
**Domain:** Backend  
**Location:** `app.go:PlayFile`

**Status:** Fixed. `PlayFile` now allows only media extensions and rejects executable/script/link-style files.

**Proof of Fix:**
```go
if !isSafePlayableFile(cleaned) {
    return errors.New("file type is not safe to play from KoalaPull")
}
```

### Yellow - Custom yt-dlp Argument Blocklist
**Severity:** MEDIUM  
**Domain:** Backend  
**Location:** `app.go:sanitizeCustomArgs`

**Status:** Fixed. Dangerous and unknown options are rejected; supported advanced args now use an allowlist.

**Proof of Fix:**
```go
requiresValue, allowed := allowedCustomArgNames[name]
if !allowed {
    return nil, fmt.Errorf("custom argument %q is not supported", name)
}
```

### Yellow - External Link Bridge Too Broad
**Severity:** MEDIUM  
**Domain:** Backend  
**Location:** `app.go:OpenExternalLink`

**Status:** Fixed. Browser-open bridge now restricts links to known project/tool hosts.

**Proof of Fix:**
```go
if !isAllowedExternalLinkHost(url) {
    return errors.New("external link host is not allowed")
}
```

### Yellow - Large Batch Import Burst
**Severity:** MEDIUM  
**Domain:** Full-Stack  
**Location:** `frontend/src/App.tsx`, `app.go:validatePlaylistItems`

**Status:** Fixed. Batch URL import and playlist item selection are capped at 25 items per action.

**Proof of Fix:**
```tsx
if (lines.length > maxBatchUrls) {
  setAddQueueError(t('errors.batchLimit', { count: maxBatchUrls }))
}
```

### Blue - UI Text, Mojibake, and Contrast
**Severity:** LOW  
**Domain:** Frontend  
**Location:** `frontend/src/App.tsx`, locale JSON, `frontend/src/style.css`

**Status:** Fixed for the audited UI areas. Hardcoded batch preset labels moved to locales, broken glyph buttons removed, and light muted text contrast improved.

**Proof of Fix:**
```css
--text-muted: #64748b;
```

## Verified Not a Code Bug

### Dependency Integrity Uses Upstream Checksums
**Severity:** INFO  
**Domain:** Infrastructure  
**Location:** `dependency_security.go`, `app.go:downloadYtdlp`, `app.go:downloadFfmpeg`

**Status:** Reclassified. Current implementation already enforces HTTPS, bounded downloads, checksum verification, private file modes, and bounded archive extraction. A fully signed or pinned supply-chain model would be stronger, but the original finding was overstated as an app vulnerability.

### Website Robots Allows Crawling
**Severity:** INFO  
**Domain:** Infrastructure  
**Location:** `website/robots.txt`

**Status:** Reclassified. This is a public marketing website SEO choice, not an application security bug. Real bot/DDoS controls belong at CDN/WAF/hosting level.

## Verification

- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `npm run test` in `frontend/` passed.
- `npm run build` in `frontend/` passed.
