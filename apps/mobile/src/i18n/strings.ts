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

/**
 * The CLDR plural category of n in lang ("one", "few", "other", …). Intl's
 * rules where the engine has them; otherwise English's, which at least gets
 * 1 right.
 */
export function pluralCategory(lang: string, n: number): string {
  try {
    return new Intl.PluralRules(lang).select(n);
  } catch {
    return n === 1 ? 'one' : 'other';
  }
}

/** Finds the brace closing the one at open; -1 if unbalanced. */
function closing(text: string, open: number): number {
  let depth = 0;
  for (let i = open; i < text.length; i++) {
    if (text[i] === '{') depth++;
    else if (text[i] === '}' && --depth === 0) return i;
  }
  return -1;
}

/**
 * Picks the form of an ICU plural block's body ("one {…} other {# …}") for
 * n: an exact "=n" form first, then n's category in lang, then "other".
 */
function pluralForm(body: string, n: number, lang: string): string | undefined {
  const forms = new Map<string, string>();
  let i = 0;
  while (i < body.length) {
    const m = /^\s*(=\d+|[a-z]+)\s*\{/.exec(body.slice(i));
    if (!m) break;
    const open = i + m[0].length - 1;
    const close = closing(body, open);
    if (close < 0) break;
    forms.set(m[1], body.slice(open + 1, close));
    i = close + 1;
  }
  const form = forms.get(`=${n}`) ?? forms.get(pluralCategory(lang, n)) ?? forms.get('other');
  return form?.replace(/#/g, String(n));
}

/**
 * Fills {name} placeholders and ICU plural blocks ("{count, plural, one
 * {Someone} other {# people}}"), whose form follows lang's plural rules.
 * Unknown placeholders are left as they are.
 */
export function format(text: string, params?: Params, lang = 'en'): string {
  if (!params) return text;
  let out = '';
  for (let i = 0; i < text.length; i++) {
    if (text[i] !== '{') {
      out += text[i];
      continue;
    }
    const plural = /^\{([a-zA-Z]+)\s*,\s*plural\s*,/.exec(text.slice(i));
    const close = closing(text, i);
    if (plural && close > 0 && typeof params[plural[1]] === 'number') {
      const form = pluralForm(text.slice(i + plural[0].length, close), params[plural[1]] as number, lang);
      if (form !== undefined) {
        out += format(form, params, lang);
        i = close;
        continue;
      }
    }
    const simple = /^\{([a-zA-Z]+)\}/.exec(text.slice(i));
    if (simple && simple[1] in params) {
      out += String(params[simple[1]]);
      i += simple[0].length - 1;
      continue;
    }
    out += text[i];
  }
  return out;
}

/**
 * Text for key, in the first of the given catalogues that has it. Callers
 * pass the language being shown first, then the other display language, so a
 * missing translation falls back to a language the person reads before
 * English, and then to plain English.
 */
export function lookup(
  key: StringKey,
  catalogs: (Strings | undefined)[],
  params?: Params,
  langs: string[] = [],
): string {
  for (const [i, c] of catalogs.entries()) {
    const text = c?.[key];
    if (text) return format(text, params, langs[i]);
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

const accented: Record<string, string> = {
  a: 'á', e: 'é', i: 'í', o: 'ö', u: 'ü', y: 'ý', c: 'ç', n: 'ñ', s: 'š', z: 'ž',
  A: 'Å', E: 'É', I: 'Î', O: 'Ø', U: 'Ü', Y: 'Ý', C: 'Ç', N: 'Ñ', S: 'Š', Z: 'Ž',
};

/**
 * A fake translation for layout checks (EXPO_PUBLIC_PSEUDO=1 in a dev
 * build): accented, about 40% longer and bracketed, so clipped or
 * overflowing text, and text that skipped t(), stand out without a real
 * language. Placeholders are kept as they are.
 */
export function pseudo(text: string): string {
  const parts = text.split(/(\{[a-zA-Z]+\})/);
  const body = parts.map((p) => (p.startsWith('{') ? p : p.replace(/[a-zA-Z]/g, (ch) => accented[ch] ?? ch))).join('');
  const letters = text.replace(/\{[a-zA-Z]+\}/g, '').length;
  return `[${body}${' ~'.repeat(Math.ceil(letters * 0.2))}]`;
}

/** English as a pseudo-translation, for layout checks. */
export const pseudoEnglish: Readonly<Record<StringKey, string>> = Object.fromEntries(
  Object.entries(displayEnglish).map(([k, v]) => [k, pseudo(v)]),
) as Record<StringKey, string>;

/**
 * The arguments of a UI string, sorted ("{name}" gives name; a plural block
 * gives its count's name), or undefined when the braces don't parse. The
 * same rule as the API's uistrings.Arguments.
 */
export function argumentNames(text: string): string[] | undefined {
  const out: string[] = [];
  for (let i = 0; i < text.length; i++) {
    if (text[i] === '}') return undefined;
    if (text[i] !== '{') continue;
    const m = /^\{([a-z][a-zA-Z]*)(\}|\s*,\s*plural\s*,)/.exec(text.slice(i));
    if (!m) return undefined;
    if (m[2] === '}') {
      out.push(m[1]);
      i += m[0].length - 1;
      continue;
    }
    const close = closing(text, i);
    if (close < 0) return undefined;
    out.push(m[1]);
    i = close;
  }
  return out.sort();
}
