# KoalaPull Website Translation Guide

The landing page copy is compiled ahead of time (AOT) from JSON files in the `locales/` directory into localized static HTML files.

---

## 🌎 Supported Locales

-   `en` (English - Root default)
-   `de` (German - Emitted to `/de/index.html`)
-   `fr` (French - Emitted to `/fr/index.html`)

---

## ⚠️ Important Rules

-   **Fallback Mechanics:** If a localized dictionary is missing a specific string key, the build compiler falls back to the English value to prevent blank layout fields.
-   **Structure Integrity:** The build process will fail if a translation file has extra keys or is missing keys compared to `en.json`. Keep keys identical across all files.
-   **Clean Copy:** Keep translations clear, brief, and aligned with technical terms used in the desktop application.
-   **Legal Regulations:** Impressum (`impressum.html`) and privacy regulations (`datenschutz.html`) are generated for German and English layouts only.

---

## 🏗️ Rebuilding Localized Layouts

1.  Add/modify dictionary values inside `website/locales/<locale_code>.json`.
2.  Compile the changes from the repository root directory:
    ```bash
    npm run build:website
    ```
3.  Open the files inside `website/www/` to verify your changes in a browser.
