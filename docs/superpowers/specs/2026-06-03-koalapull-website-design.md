# KoalaPull Website Design

## Goal

Build a static marketing website for KoalaPull inside `website/` that mirrors the proven KoalaPlay website structure and visual style closely enough for a first release, while adapting the content to KoalaPull's desktop-app use case.

## Scope

The website should:

- explain what KoalaPull is
- show why users should use it
- link to GitHub, releases, and downloads
- support three languages: English, German, French
- include a visual click-dummy style mockup of the app
- include feature highlights, FAQ, and usage explanation
- compile from one template plus translation dictionaries into static output under `website/www/`

The website should not:

- depend on a runtime backend
- embed the real React app
- require external CDNs for core assets
- attempt perfect final copywriting or perfect visual polish in this pass

## Reference Inputs

### KoalaPlay Reference

Reuse the KoalaPlay website architecture and most of its implementation approach:

- `website/template.html`
- `website/style.css`
- `website/app.js`
- `website/lang-init.js`
- `website/build.js`
- locale-driven static generation into `website/www/`

### KoalaPull Product Inputs

Content should be derived from:

- `README.md`
- `frontend/src/App.tsx`
- `frontend/src/locales/en.json`
- `frontend/src/locales/de.json`
- `frontend/src/locales/fr.json`

These sources define the app's feature set, language coverage, supported platforms, setup behavior, and user-facing wording.

## Architecture

### Directory Layout

Introduce a new `website/` tree in KoalaPull with this structure:

```text
website/
├── README.md
├── TRANSLATION.md
├── template.html
├── style.css
├── app.js
├── lang-init.js
├── build.js
├── robots.txt
├── site.webmanifest
├── sitemap.xml
├── version.json
├── assets/
├── locales/
│   ├── en.json
│   ├── de.json
│   └── fr.json
└── www/
    └── ...generated files...
```

### Build Model

The source of truth is `website/template.html` plus locale JSON dictionaries.

`website/build.js` will:

1. load the shared template
2. load `en`, `de`, and `fr` locale dictionaries
3. inject translation keys into localized HTML output
4. minify CSS and JS
5. fingerprint built CSS and JS filenames
6. emit fully static localized files into `website/www/`

Output behavior:

- English root page at `website/www/index.html`
- localized subpages at `website/www/de/index.html` and `website/www/fr/index.html`
- shared hashed assets in `website/www/`

## Page Structure

### Hero

The hero section should explain KoalaPull as a native desktop download manager for `yt-dlp`, aimed at people who want download power without terminal complexity.

Primary calls to action:

- GitHub repository
- GitHub releases
- direct platform/release hint text

Secondary proof points:

- open source
- privacy-first
- cross-platform

### App Mockup

The page should include a click-dummy style mockup inspired by the KoalaPlay site, but adapted to KoalaPull.

The mockup should visually represent:

- URL input
- metadata preview card
- format choice
- queue/download list
- settings/help feel

Interaction can be lightweight and fake:

- switch tabs or panels
- toggle download presets
- animate progress

No real app state or backend integration is required.

### Benefit Sections

Sections should communicate:

- no terminal needed
- automatic dependency setup
- metadata preview before download
- playlists and queues
- safe local-first behavior
- familiar desktop workflow

### Feature Blocks

Feature blocks should map closely to README and frontend capabilities:

- dependency auto-setup
- metadata preview
- format selection
- playlist support
- concurrent downloads
- history
- theme support
- updates
- popular supported sites

### How It Works

A short step-based explanation:

1. paste URL
2. fetch metadata
3. choose format
4. add to queue
5. open finished file from output folder

### FAQ

FAQ should cover likely user questions:

- is KoalaPull free and open source
- does it need yt-dlp or ffmpeg installed first
- what sites are supported
- can it download playlists
- is it private
- why use this instead of terminal commands

## Localization

### Supported Languages

This first pass supports:

- `en`
- `de`
- `fr`

### Translation Strategy

Use the KoalaPlay static compilation approach:

- all homepage copy in locale JSON dictionaries
- no client-side runtime translation dependency for main pages
- language switcher rewrites to static localized paths

Translation keys should cover:

- SEO/meta fields
- nav labels
- hero text
- benefit cards
- feature lists
- mockup labels
- FAQ
- footer text

## Styling Direction

Reuse KoalaPlay styling heavily for speed and consistency:

- same layout rhythm
- same broad visual language
- same animation/reveal patterns
- same mobile nav behavior

Adapt branding and copy for KoalaPull:

- KoalaPull naming everywhere
- desktop-download product framing instead of browser-sync framing
- asset references updated to KoalaPull-specific visuals where available

If KoalaPull lacks enough custom art right now, placeholder or repurposed local assets are acceptable for this pass as long as the site remains coherent.

## JavaScript Behavior

`website/app.js` should remain lightweight.

Planned behavior:

- reveal-on-scroll animation
- sticky nav polish
- smooth scrolling
- mockup tab switching or state toggles
- language select navigation

No analytics, no trackers, no network-heavy behavior.

## SEO and Static Metadata

Include:

- title and description per locale
- canonical URLs
- hreflang links for `en`, `de`, `fr`
- Open Graph and Twitter metadata
- schema blocks for software app and FAQ if practical to adapt from KoalaPlay

Exact production domain can remain placeholder-friendly if the final domain is not yet established, but the structure should support later replacement from one place.

## Testing and Verification

Implementation verification should include:

- `node website/build.js`
- inspect generated files under `website/www/`
- verify no broken template placeholders remain
- verify language switching paths
- verify mockup JS works without console errors

Before any push, follow repository quality gate:

- `./scripts/verify.sh`
- `go test -count=1 ./...`
- `go vet ./...`
- `npm run test`
- `npm run build`

For this website task, if push is not requested, build-specific verification is still required before claiming completion.

## Risks and Tradeoffs

### Strengths of This Approach

- fastest path to a solid first website
- proven static architecture already exists nearby
- easy multi-language growth later
- easy deployment to static hosting

### Accepted Tradeoffs

- visual identity will be close to KoalaPlay at first
- copy may need a later polish pass
- mockup is illustrative, not a live embedded app

## Implementation Boundary

This spec covers only the website source/build system and generated static site output inside KoalaPull.

It does not include:

- production deployment setup
- legal pages unless later requested
- screenshot automation
- app-store style distribution pages
