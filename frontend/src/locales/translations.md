# Translation Status & Contribution Guide

KoalaPull supports multiple interface languages. The primary locale dictionary is `en.json`, which serves as the source of truth for all translation keys.

---

## 📊 Translation Status

| Language | Locale File | Coverage | Status | Validation |
|---|---|---|---|---|
| **English** | `en.json` | 100% | Source | Verified |
| **German** | `de.json` | 100% | Translated | Verified |
| **French** | `fr.json` | 100% | Translated | Needs Native Verification |

---

## ✍️ How to Add a New Translation

1.  **Create the Locale File:**
    Navigate to `frontend/src/locales/` and create `<locale_code>.json` (for example, `es.json` for Spanish).

2.  **Translate Content:**
    Copy the structure of `en.json` and translate the values only.
    > [!IMPORTANT]
    > Do not delete, rename, or reorder JSON keys. The keys must match `en.json` exactly to prevent application layout breaking or missing translation errors.

3.  **Register the Translation:**
    Register the locale inside `frontend/src/lib/i18n.ts` so the system detects and loads it.

4.  **Test Locally:**
    Launch the application using `wails dev`, open **Settings**, change the language drop-down menu, and verify the text changes correctly on:
    -   Active downloads queue and format dropdowns.
    -   Download settings and folders.
    -   History logs and the Help section.

5.  **Submit a Pull Request:**
    Open a pull request containing:
    -   The new `<locale_code>.json` translation dictionary.
    -   The registration edits in `i18n.ts`.
    -   An updated status table in this document.
    -   A brief comment stating whether the translation was written by a native speaker or generated/assisted by machine translators.
