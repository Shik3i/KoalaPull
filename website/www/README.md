# KoalaPull Website

Static site for KoalaPull.

## What lives here

- `build.js` compiles the site into `website/www/`
- `template.html` is the source page shell
- `locales/` holds the `en`, `de`, and `fr` copy
- `robots.txt`, `sitemap.xml`, `site.webmanifest`, and `version.json` are copied into the build output

## Build

```bash
npm run build:website
```

The site is static only. No backend.

## Notes

- Use placeholder domain `https://pull.koalastuff.net/` until the final host is set.
- GitHub links point to `https://github.com/Shik3i/KoalaPull`.
- Do not edit generated files in `website/www/`.
