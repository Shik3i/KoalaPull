# KoalaPull Website Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a static, multi-language KoalaPull landing page in `website/` with a KoalaPlay-style layout, locale compilation, hashed/minified assets, and automatic PNG-to-WebP/AVIF asset generation.

**Architecture:** Copy the KoalaPlay website pipeline as the base, then replace product copy and mockup markup for KoalaPull. Keep the source site in plain HTML/CSS/JS plus locale JSON, and compile it into `website/www/` with one Node build script that minifies assets, fingerprints outputs, and generates responsive AVIF/WebP images from source PNG assets.

**Tech Stack:** Static HTML, CSS, vanilla JavaScript, Node.js build script, `sharp` for image conversion, local JSON locales

---

## File Map

- Create: `website/README.md`
- Create: `website/TRANSLATION.md`
- Create: `website/template.html`
- Create: `website/style.css`
- Create: `website/app.js`
- Create: `website/lang-init.js`
- Create: `website/build.js`
- Create: `website/robots.txt`
- Create: `website/site.webmanifest`
- Create: `website/sitemap.xml`
- Create: `website/version.json`
- Create: `website/www/README.md`
- Create: `website/locales/en.json`
- Create: `website/locales/de.json`
- Create: `website/locales/fr.json`
- Create: `website/assets/` generated and copied website assets
- Modify or create: `package.json`
- Modify or create: `package-lock.json`
- Reuse source image: `assets/Icon.png`

## Task 1: Add Website Build Dependencies

**Files:**
- Modify or create: `package.json`
- Modify or create: `package-lock.json`

- [ ] **Step 1: Add a root `package.json` with website build dependencies**

```json
{
  "name": "koalapull",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "build:website": "node website/build.js"
  },
  "devDependencies": {
    "sharp": "^0.34.5"
  }
}
```

- [ ] **Step 2: Install dependencies and create `package-lock.json`**

Run: `npm install`
Expected: `added ... packages` and a new `package-lock.json`

- [ ] **Step 3: Verify `sharp` resolves**

Run: `node -e "require('sharp'); console.log('sharp ok')"`
Expected: `sharp ok`

- [ ] **Step 4: Commit**

```bash
git add package.json package-lock.json
git commit -m "build: add website build dependencies"
```

## Task 2: Copy KoalaPlay Website Skeleton Into KoalaPull

**Files:**
- Create: `website/README.md`
- Create: `website/TRANSLATION.md`
- Create: `website/template.html`
- Create: `website/style.css`
- Create: `website/app.js`
- Create: `website/lang-init.js`
- Create: `website/build.js`
- Create: `website/robots.txt`
- Create: `website/site.webmanifest`
- Create: `website/sitemap.xml`
- Create: `website/version.json`
- Create: `website/www/README.md`

- [ ] **Step 1: Copy the KoalaPlay website source files as the starting point**

Copy these files from `/Users/koala/Documents/KoalaPlay/website/`:

```text
README.md
TRANSLATION.md
template.html
style.css
app.js
lang-init.js
build.js
robots.txt
site.webmanifest
sitemap.xml
version.json
www/README.md
```

- [ ] **Step 2: Rename product references in documentation and metadata files**

Replace `KoalaSync` with `KoalaPull` in:

```text
website/README.md
website/TRANSLATION.md
website/version.json
website/site.webmanifest
website/sitemap.xml
```

Also replace product-domain assumptions with temporary KoalaPull placeholders such as:

```text
https://pull.koalastuff.net/
https://github.com/Shik3i/KoalaPull
```

- [ ] **Step 3: Strip KoalaSync-only utility page references**

Remove or avoid references to these pages from the KoalaPull website system:

```text
join.html
impressum.html
datenschutz.html
bridge flow
extension detection
Firefox/Chrome store CTAs
```

- [ ] **Step 4: Run a placeholder scan**

Run: `rg -n "KoalaSync|join\\.html|addons\\.mozilla|chromewebstore|Teleparty|Netflix|Jellyfin|Emby" website`
Expected: only intentional leftover references, or no matches after cleanup

- [ ] **Step 5: Commit**

```bash
git add website
git commit -m "feat: scaffold koalapull website system"
```

## Task 3: Add Website Asset Pipeline From `assets/Icon.png`

**Files:**
- Modify: `website/build.js`
- Create: `website/assets/` outputs

- [ ] **Step 1: Add source-image generation rules to `website/build.js`**

Use `assets/Icon.png` as the master source. Generate these variants during the build:

```js
const imageSpecs = [
  { name: 'NewLogoIcon', width: 200, format: 'webp' },
  { name: 'NewLogoIcon_128', width: 128, format: 'webp' },
  { name: 'NewLogoIcon_64', width: 64, format: 'webp' },
  { name: 'NewLogoIcon', width: 200, format: 'avif' },
  { name: 'NewLogoIcon_128', width: 128, format: 'avif' },
  { name: 'NewLogoIcon_64', width: 64, format: 'avif' }
];
```

And for a hero/support visual based on the same source PNG:

```js
const responsiveSpecs = [
  { name: 'IconHero-1x', width: 180 },
  { name: 'IconHero', width: 360 }
];
```

For each responsive spec, emit:

```text
IconHero-1x.webp
IconHero.webp
IconHero-1x.avif
IconHero.avif
```

- [ ] **Step 2: Copy PNG originals needed for icons**

Ensure build output still includes:

```text
favicon-16x16.png
favicon-32x32.png
icon-192x192.png
apple-touch icon target
```

These can come from resized `assets/Icon.png` during the build.

- [ ] **Step 3: Reuse the KoalaPlay `<picture>` injection pattern**

Extend the build output transform so `<img>` tags that target `.webp` assets become:

```html
<picture>
  <source srcset="assets/IconHero-1x.avif 180w, assets/IconHero.avif 360w" type="image/avif">
  <img src="assets/IconHero-1x.webp" srcset="assets/IconHero-1x.webp 180w, assets/IconHero.webp 360w" sizes="180px" alt="...">
</picture>
```

- [ ] **Step 4: Run the build script once and inspect generated assets**

Run: `node website/build.js`
Expected: generated `.webp`, `.avif`, and `.png` files in `website/www/assets/`

- [ ] **Step 5: Commit**

```bash
git add website/build.js website/assets website/www
git commit -m "build: add koalapull website image pipeline"
```

## Task 4: Replace Template Content With KoalaPull Landing Page Sections

**Files:**
- Modify: `website/template.html`

- [ ] **Step 1: Replace hero and nav content**

Set the page sections and anchor IDs to:

```html
<a href="#features"><span>{{NAV_FEATURES}}</span></a>
<a href="#mockup"><span>{{NAV_MOCKUP}}</span></a>
<a href="#how-it-works"><span>{{NAV_HOW_IT_WORKS}}</span></a>
<a href="#faq"><span>{{NAV_FAQ}}</span></a>
<a href="https://github.com/Shik3i/KoalaPull" target="_blank">GitHub</a>
```

Hero copy should use translation keys such as:

```html
<h1 data-reveal>{{HERO_TITLE}}</h1>
<p class="hero-subtitle" data-reveal>{{HERO_SUBTITLE}}</p>
```

- [ ] **Step 2: Replace extension mockup with KoalaPull app mockup**

Use a static app-like mockup block with sections for:

```html
<section id="mockup" class="mockup-section">
  <div class="app-window">
    <div class="app-sidebar">
      <button data-mock-tab="downloads">{{MOCK_TAB_DOWNLOADS}}</button>
      <button data-mock-tab="history">{{MOCK_TAB_HISTORY}}</button>
      <button data-mock-tab="settings">{{MOCK_TAB_SETTINGS}}</button>
      <button data-mock-tab="help">{{MOCK_TAB_HELP}}</button>
    </div>
    <div class="app-panel" data-mock-panel="downloads">...</div>
    <div class="app-panel" data-mock-panel="history" hidden>...</div>
    <div class="app-panel" data-mock-panel="settings" hidden>...</div>
    <div class="app-panel" data-mock-panel="help" hidden>...</div>
  </div>
</section>
```

Populate the mockup with fake but product-correct values:

```html
<input value="https://www.youtube.com/watch?v=koalapull-demo" readonly>
<div class="meta-card">...</div>
<div class="queue-card">...</div>
```

- [ ] **Step 3: Replace KoalaSync-specific sections with KoalaPull content**

Keep the structure but rewrite the sections to:

```text
Why KoalaPull
Popular features
How it works
FAQ
GitHub CTA
```

Feature cards should represent:

```text
Auto-setup
Metadata preview
Queue and concurrency
Playlists
History
Themes and updates
```

- [ ] **Step 4: Simplify footer and remove unsupported legal links**

Keep:

```html
<a href="https://github.com/Shik3i/KoalaPull">GitHub</a>
<a href="https://github.com/Shik3i/KoalaPull/releases">Releases</a>
```

Do not link pages that do not exist in KoalaPull yet.

- [ ] **Step 5: Run a content sanity scan**

Run: `rg -n "KoalaSync|extension|join room|Firefox Add-ons|Chrome Web Store|Teleparty|sync videos" website/template.html`
Expected: no stale product wording

- [ ] **Step 6: Commit**

```bash
git add website/template.html
git commit -m "feat: adapt website template for koalapull"
```

## Task 5: Add KoalaPull Locale Dictionaries

**Files:**
- Create: `website/locales/en.json`
- Create: `website/locales/de.json`
- Create: `website/locales/fr.json`

- [ ] **Step 1: Create the English source dictionary**

Use keys like:

```json
{
  "LANG_CODE": "en",
  "HTML_CLASS": "lang-en",
  "CANONICAL_PATH": "",
  "LANG_TOGGLE_URL": "de/",
  "LANG_TOGGLE_TEXT": "DE",
  "META_TITLE": "KoalaPull - Native yt-dlp download manager",
  "META_DESCRIPTION": "Download videos, audio, playlists, subtitles, and metadata with a clean desktop app for macOS, Windows, and Linux.",
  "NAV_FEATURES": "Features",
  "NAV_MOCKUP": "Mockup",
  "NAV_HOW_IT_WORKS": "How It Works",
  "NAV_FAQ": "FAQ",
  "HERO_TITLE": "The easy desktop face of yt-dlp.",
  "HERO_SUBTITLE": "Paste a link, preview the media, choose a format, and let KoalaPull handle the queue."
}
```

- [ ] **Step 2: Fill feature, FAQ, and mockup keys from KoalaPull sources**

Base wording on:

```text
README.md
frontend/src/locales/en.json
frontend/src/locales/de.json
frontend/src/locales/fr.json
```

Add keys for:

```text
feature cards
mockup tab labels
mockup queue labels
FAQ questions and answers
CTA labels
footer labels
schema/how-to fields
```

- [ ] **Step 3: Translate German and French locale files**

Use:

```text
website/locales/de.json
website/locales/fr.json
```

Keep key shape identical to English.

- [ ] **Step 4: Validate locale key parity**

Run: `node -e "const fs=require('fs'); const p='website/locales'; const base=Object.keys(JSON.parse(fs.readFileSync(p+'/en.json','utf8'))).sort(); for (const f of ['de.json','fr.json']) { const keys=Object.keys(JSON.parse(fs.readFileSync(p+'/'+f,'utf8'))).sort(); console.log(f, JSON.stringify(keys)===JSON.stringify(base) ? 'ok' : 'mismatch'); }"`
Expected: `de.json ok` and `fr.json ok`

- [ ] **Step 5: Commit**

```bash
git add website/locales
git commit -m "feat: add koalapull website locales"
```

## Task 6: Adapt Website Styles for KoalaPull Mockup and Sections

**Files:**
- Modify: `website/style.css`

- [ ] **Step 1: Keep KoalaPlay global style base**

Retain:

```css
:root { ... }
body { ... }
nav { ... }
.container { ... }
.hero { ... }
.btn { ... }
[data-reveal] { ... }
```

- [ ] **Step 2: Remove unused extension/join-page styles**

Delete or ignore styles tied only to:

```css
.join-*
.browser-compat-*
.invite-banner
.extension-*
```

when they are no longer used by the new HTML.

- [ ] **Step 3: Add KoalaPull mockup styles**

Introduce selectors like:

```css
.app-window { ... }
.app-sidebar { ... }
.app-panel { ... }
.meta-card { ... }
.queue-card { ... }
.history-row { ... }
.settings-grid { ... }
.status-pill { ... }
```

Keep the visual direction close to KoalaPlay:

```css
background: rgba(15, 23, 42, 0.7);
border: 1px solid rgba(148, 163, 184, 0.2);
backdrop-filter: blur(16px);
```

- [ ] **Step 4: Verify mobile layout behavior**

Run: `rg -n "@media|app-window|app-sidebar|mockup-section" website/style.css`
Expected: explicit responsive rules for the mockup and content sections

- [ ] **Step 5: Commit**

```bash
git add website/style.css
git commit -m "style: adapt koalapull website styles"
```

## Task 7: Replace Website JavaScript With KoalaPull Mockup Interactions

**Files:**
- Modify: `website/app.js`
- Modify: `website/lang-init.js` if needed

- [ ] **Step 1: Keep shared UX helpers**

Retain:

```js
IntersectionObserver reveal logic
sticky nav behavior
smooth anchor scrolling
language dropdown navigation
mobile nav toggle
```

- [ ] **Step 2: Remove KoalaSync invite and extension bridge logic**

Delete blocks related to:

```js
checkInvite()
KOALASYNC_STATUS
KOALASYNC_JOIN_REQUEST
join page DOM mutations
extension installed dataset
```

- [ ] **Step 3: Add simple mockup tab switching**

Implement:

```js
const tabButtons = document.querySelectorAll('[data-mock-tab]');
const panels = document.querySelectorAll('[data-mock-panel]');

tabButtons.forEach((button) => {
  button.addEventListener('click', () => {
    const tab = button.dataset.mockTab;
    tabButtons.forEach((item) => item.classList.toggle('is-active', item === button));
    panels.forEach((panel) => {
      panel.hidden = panel.dataset.mockPanel !== tab;
    });
  });
});
```

- [ ] **Step 4: Add lightweight animated progress state**

Use a tiny loop for one or two fake progress bars:

```js
const bars = document.querySelectorAll('[data-progress-fill]');
let pct = 42;
setInterval(() => {
  pct = pct >= 91 ? 42 : pct + 1;
  bars.forEach((bar) => {
    bar.style.width = `${pct}%`;
  });
}, 1200);
```

- [ ] **Step 5: Build and inspect for stale product strings**

Run: `rg -n "KOALASYNC|join|extension|chromewebstore|addons.mozilla" website/app.js website/lang-init.js`
Expected: no stale extension-only logic

- [ ] **Step 6: Commit**

```bash
git add website/app.js website/lang-init.js
git commit -m "feat: add koalapull website interactions"
```

## Task 8: Finalize Build Output, Metadata, and Documentation

**Files:**
- Modify: `website/build.js`
- Modify: `website/README.md`
- Modify: `website/TRANSLATION.md`
- Modify: `website/robots.txt`
- Modify: `website/site.webmanifest`
- Modify: `website/sitemap.xml`
- Modify: `website/version.json`

- [ ] **Step 1: Update build language array to exactly three languages**

Use:

```js
const languages = ['en', 'de', 'fr'];
```

- [ ] **Step 2: Set KoalaPull-specific metadata**

Replace schema and manifest values with:

```text
SoftwareApplication
name: KoalaPull
sameAs: https://github.com/Shik3i/KoalaPull
applicationCategory: DesktopApplication
operatingSystem: macOS, Windows, Linux
```

- [ ] **Step 3: Update website docs for the KoalaPull workflow**

Document:

```text
edit source files only
run node website/build.js
source icon comes from assets/Icon.png
build emits 1x/2x webp and avif variants automatically
```

- [ ] **Step 4: Rebuild website and inspect output**

Run: `node website/build.js`
Expected: fresh `website/www/` with:

```text
index.html
de/index.html
fr/index.html
hashed CSS/JS
generated assets
```

- [ ] **Step 5: Commit**

```bash
git add website
git commit -m "docs: finalize koalapull website build output"
```

## Task 9: Verify Website Deliverable

**Files:**
- Verify: `website/www/index.html`
- Verify: `website/www/de/index.html`
- Verify: `website/www/fr/index.html`

- [ ] **Step 1: Run the website build**

Run: `node website/build.js`
Expected: successful build summary with no missing locale or asset errors

- [ ] **Step 2: Check for leftover template placeholders**

Run: `rg -n "{{[A-Z0-9_\\-]+}}" website/www`
Expected: no matches

- [ ] **Step 3: Check generated language pages**

Run: `find website/www -maxdepth 2 -name 'index.html' | sort`
Expected:

```text
website/www/de/index.html
website/www/fr/index.html
website/www/index.html
```

- [ ] **Step 4: Check generated image variants**

Run: `find website/www/assets -maxdepth 1 \\( -name '*.webp' -o -name '*.avif' -o -name '*.png' \\) | sort`
Expected: `NewLogoIcon*`, `IconHero*`, and favicon/icon PNG files

- [ ] **Step 5: Run repository verification gate before claiming completion**

Run: `./scripts/verify.sh`
Expected: success

Run: `go test -count=1 ./...`
Expected: success

Run: `go vet ./...`
Expected: success

Run: `npm run test --prefix frontend`
Expected: success

Run: `npm run build --prefix frontend`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add website/www
git commit -m "test: verify koalapull website build"
```

## Self-Review Notes

- Spec coverage: covered website scaffold, three locales, mockup, FAQ, GitHub links, build system, hashed assets, and image conversion from `assets/Icon.png`.
- Placeholder scan: no `TODO`, `TBD`, or vague handoffs remain.
- Type consistency: mockup selectors use `data-mock-tab`, `data-mock-panel`, and `data-progress-fill` consistently across HTML, CSS, and JS tasks.
