# KoalaPull Translation Guide

KoalaPull website copy is compiled ahead of time from locale JSON files.

## Active locales

- `en`
- `de`
- `fr`

## Rules

- Keep locale keys in sync across all three files.
- Use short, product-focused copy.
- Keep legal or policy text out of the site for now.

## Build flow

1. Write or update `website/locales/*.json`
2. Run `npm run build:website`
3. Inspect `website/www/`

## URLs

- Placeholder site domain: `https://pull.koalastuff.net/`
- Project repo: `https://github.com/Shik3i/KoalaPull`
