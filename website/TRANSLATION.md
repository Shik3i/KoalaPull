# KoalaPull Translation Guide

KoalaPull website copy is compiled ahead of time from locale JSON files.

## Active locales

- `en`
- `de`
- `fr`

## Rules

- Missing landing-page keys fall back to English during build.
- Use short, product-focused copy.
- Legal pages are generated only as English `imprint.html` / `privacy.html` and German `de/impressum.html` / `de/datenschutz.html`.
- Unknown or unsupported language paths should fall back to English unless German is explicitly selected.
- The build now fails on missing or extra locale keys.

## Build flow

1. Write or update `website/locales/*.json`
2. Run `npm run build:website`
3. Inspect `website/www/`

## URLs

- Placeholder site domain: `https://pull.koalastuff.net/`
- Project repo: `https://github.com/Shik3i/KoalaPull`
