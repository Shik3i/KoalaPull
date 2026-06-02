# Translation Status

`en.json` is the source locale. Every other language file must keep the exact same keys.

| Language | File | Coverage | Source | Verification |
| --- | --- | --- | --- | --- |
| English | `en.json` | 100% | source | verified |
| German | `de.json` | 100% | translated | needs verification |
| French | `fr.json` | 100% | machine-assisted | needs verification |

# Contribution Guide

Add new languages in `frontend/src/locales/` as `<lang>.json`.

Keep every key exactly the same as `en.json`.

Translate values only. Do not rename, remove, or reorder keys just to make review easier.

Test the new language in the app by switching it in Settings and checking at least:

- Downloads
- Settings
- One secondary screen such as History or Help

Update the table above with the new language, current coverage, and verification state.

Open a pull request that includes:

- the new locale file
- the updated `translations.md`
- screenshots of the Settings screen and at least one other localized screen
- a short note saying whether the translation is native-reviewed or machine-assisted

If a string is unclear, keep the English meaning consistent instead of inventing a new one.
