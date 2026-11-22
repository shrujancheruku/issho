import en from './en.json';
import vi from './vi.json';

/**
 * The app's languages: the languages of the open cities, a short fixed list
 * (docs/spec/languages.md). A language is added with its JSON file, here and
 * in scripts/check-strings.js.
 */
export const languages = ['en', 'vi'] as const;
export type Language = (typeof languages)[number];
export type StringKey = keyof typeof en;

const catalogues: Record<Language, Record<StringKey, string>> = { en, vi };

/** The first of the device's languages (BCP 47 tags, best first) on the list; English when none is. */
export function pickLanguage(tags: readonly string[]): Language {
  for (const tag of tags) {
    const base = tag.split('-')[0].toLowerCase();
    const found = languages.find((language) => language === base);
    if (found) return found;
  }
  return 'en';
}

/** `template` with each `{name}` replaced from `params`; a name with no value is left as written. */
export function fill(template: string, params: Record<string, string | number> = {}): string {
  return template.replace(/\{(\w+)\}/g, (whole, name: string) => (name in params ? String(params[name]) : whole));
}

/** The text for `key` in `lang`. */
export function translate(lang: Language, key: StringKey, params?: Record<string, string | number>): string {
  return fill(catalogues[lang][key], params);
}
