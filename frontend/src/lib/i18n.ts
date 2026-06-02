import de from "../locales/de.json"
import en from "../locales/en.json"
import fr from "../locales/fr.json"

export type LanguageCode = "en" | "de" | "fr"

type LocaleObject = Record<string, unknown>
type TranslateParams = Record<string, string | number>
type TranslateFn = (key: string, params?: TranslateParams) => string

const locales: Record<LanguageCode, LocaleObject> = { en, de, fr }

function getNestedValue(locale: LocaleObject, key: string): unknown {
  return key.split(".").reduce<unknown>((current, part) => {
    if (!current || typeof current !== "object" || Array.isArray(current)) {
      return undefined
    }
    return (current as LocaleObject)[part]
  }, locale)
}

function interpolate(template: string, params?: TranslateParams): string {
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (_, key) => String(params[key] ?? `{${key}}`))
}

export function createTranslator(language: LanguageCode): TranslateFn {
  return (key, params) => translate(language, key, params)
}

export function translate(
  language: LanguageCode,
  key: string,
  params?: TranslateParams,
  localeSet: Record<LanguageCode, LocaleObject> = locales,
): string {
  const selected = getNestedValue(localeSet[language], key)
  const fallback = getNestedValue(localeSet.en, key)
  const resolved = typeof selected === "string" ? selected : typeof fallback === "string" ? fallback : key
  return interpolate(resolved, params)
}

export function getLocaleKeys(locale: LocaleObject, prefix = ""): string[] {
  return Object.entries(locale)
    .flatMap(([key, value]) => {
      const path = prefix ? `${prefix}.${key}` : key
      if (!value || typeof value !== "object" || Array.isArray(value)) {
        return [path]
      }
      return getLocaleKeys(value as LocaleObject, path)
    })
    .sort()
}

export function isSupportedLanguage(value: string): value is LanguageCode {
  return value === "en" || value === "de" || value === "fr"
}

export function getLanguageLocale(language: LanguageCode): string {
  switch (language) {
    case "de":
      return "de-DE"
    case "fr":
      return "fr-FR"
    default:
      return "en-US"
  }
}

export const localeRegistry = locales
