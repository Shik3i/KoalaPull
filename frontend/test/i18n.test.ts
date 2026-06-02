import { describe, expect, it } from "vitest"
import de from "../src/locales/de.json"
import en from "../src/locales/en.json"
import fr from "../src/locales/fr.json"
import { getLocaleKeys, translate, localeRegistry } from "../src/lib/i18n"

describe("i18n", () => {
  it("falls back to english and interpolates params", () => {
    const customLocales = {
      ...localeRegistry,
      fr: {
        ...fr,
        downloads: {},
      },
    }

    expect(translate("fr", "downloads.summary", { count: 3 }, customLocales)).toBe("Downloads (3)")
    expect(translate("fr", "common.currentVersion", { version: "v1.2.3" }, customLocales)).toBe("(actuelle : v1.2.3)")
  })

  it("keeps locale key sets aligned with english", () => {
    const englishKeys = getLocaleKeys(en)

    expect(getLocaleKeys(de)).toEqual(englishKeys)
    expect(getLocaleKeys(fr)).toEqual(englishKeys)
  })
})
