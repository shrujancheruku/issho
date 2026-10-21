import en from './en.json';

/**
 * The app's UI text is written in English in en.json. Every other language is
 * machine-translated on the server (UIService.GetStrings), cached there for
 * everyone and on the device (cache.ts). en.json is also copied into the API
 * by `make gen`: the server only translates strings it knows.
 */
export const english: Readonly<Record<StringKey, string>> = en;
export type StringKey = keyof typeof en;
export type Strings = Partial<Record<string, string>>;
export type Params = Record<string, string | number>;

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
 * English.
 */
export function lookup(key: StringKey, catalogs: (Strings | undefined)[], params?: Params): string {
  for (const c of catalogs) {
    const text = c?.[key];
    if (text) return format(text, params);
  }
  return format(english[key], params);
}

/** True when a cached catalogue has every key this build of the app uses. */
export function coversApp(strings: Strings): boolean {
  return (Object.keys(english) as StringKey[]).every((k) => !!strings[k]);
}

/** "ja-JP" → "ja": translations are per language, not per region. */
export function baseLanguage(tag: string | null | undefined): string {
  return (tag ?? '').trim().toLowerCase().split(/[-_]/)[0];
}
