# KoalaPull Website Project

This folder contains the static marketing landing page and assets for KoalaPull.

---

## 📁 Project Structure

-   `build.js`: Node compilation script. Resolves locales, processes images, hashes/minifies styles/scripts, and writes the output directory.
-   `template.html`: Base HTML layout file.
-   `style.css`: Base visual stylesheet.
-   `app.js` / `lang-init.js`: Frontend script utilities.
-   `locales/`: Active landing page JSON dictionaries.
-   `robots.txt` / `sitemap.xml` / `site.webmanifest` / `version.json`: Search engine optimization files and app manifest definitions.
-   `www/`: The compiled, self-contained output folder.

---

## 🛠️ Build Operations

Before compiling the site, make sure you have installed the project devDependencies from the repository root:
```bash
npm install
```

To compile the templates and generate the output website files, run the build script:
```bash
npm run build:website
```
This runs the local Node build process:
1.  Loads the template HTML file.
2.  Combines the JSON dictionaries located in `locales/` into translated static page targets (`index.html`, `de/index.html`, `fr/index.html`).
3.  Minifies and fingerprints JS and CSS files with cache-busting hashes.
4.  Processes source images, converting assets into WebP and AVIF formats using `sharp`.
5.  Emits all ready-to-deploy static assets inside `website/www/`.

---

## ⚠️ Notes for Developers

-   **Do not edit files in `website/www/` directly:** The `www/` folder is generated dynamically during builds. Any changes made inside it will be overwritten. Always edit the files in the root of `website/` or `website/locales/` and run the compiler script.
-   **Resource Links:** GitHub repository references point to `https://github.com/Shik3i/KoalaPull`.
-   **Static Domain:** The default deployment host points to `https://pull.koalastuff.net/` until production setups are finalized.
