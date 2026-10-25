import en from './en.json';
import enHuman from './human/en.json';

/**
 * The app's UI text is written in plain English in en.json. Every other
 * language is machine-translated from it on the server (UIService.GetStrings),
 * cached there for everyone and on the device (cache.ts). en.json is also
 * copied into the API by `make gen`: the server only translates strings it
 * knows.
 */
export const english: Readonly<Record<StringKey, string>> = en;
export type StringKey = keyof typeof en;
export type Strings = Partial<Record<string, string>>;
export type Params = Record<string, string | number>;

/** A person's version of one string (see backend/internal/uistrings/human.go). */
type Human = { text: string; source: string };

/**
 * human/en.json: livelier English for some strings, shown to people who use
 * the app in English. Machines still translate the plain text, and the plain
 * text stays the last resort in lookup (whoever gets it may not read English
 * well). CI fails while an entry was written for older English text, so the
 * bundled versions are always current. Other languages' human versions come
 * merged from the server.
 */
export const englishHuman: Readonly<Partial<Record<string, Human>>> = enHuman;

/** English as shown: the human version where there is one. */
export const displayEnglish: Readonly<Record<StringKey, string>> = {
  ...english,
  ...Object.fromEntries(
    Object.entries(englishHuman).flatMap(([key, v]) => (key in english && v?.text ? [[key, v.text]] : [])),
  ),
};

/** Fills {name} placeholders. Unknown placeholders are left as they are. */
export function format(text: string, params?: Params): string {
  if (!params) return text;
  return text.replace(/\{([a-zA-Z]+)\}/g, (whole, name: string) =>
    name in params ? String(params[name]) : whole,
  );
}

/**
 * Text for key, in the first of the given catalogues that has it. Callers
 * pass the language being shown first, then the other display language, so a
 * missing translation falls back to a language the person reads before
 * English, and then to plain English.
 */
export function lookup(key: StringKey, catalogs: (Strings | undefined)[], params?: Params): string {
  for (const c of catalogs) {
    const text = c?.[key];
    if (text) return format(text, params);
  }
  return format(english[key], params);
}

/**
 * A fingerprint of this build's English text (FNV-1a over en.json). Cached
 * translations record the fingerprint they were fetched for, so rewording an
 * English string (same key, new text) refetches instead of keeping the old
 * translation forever. Not security-relevant; it only has to change when the
 * text does.
 */
export function fingerprint(catalog: Readonly<Record<string, string>>): string {
  let h = 0x811c9dc5;
  const text = JSON.stringify(Object.entries(catalog).sort(([a], [b]) => (a < b ? -1 : 1)));
  for (let i = 0; i < text.length; i++) {
    h ^= text.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return (h >>> 0).toString(16);
}

export const catalogFingerprint = fingerprint(english);

/** "ja-JP" → "ja": translations are per language, not per region. */
export function baseLanguage(tag: string | null | undefined): string {
  return (tag ?? '').trim().toLowerCase().split(/[-_]/)[0];
}
