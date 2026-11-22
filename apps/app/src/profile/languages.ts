import { LANGUAGE_NAMES } from './language-names';

/** `code`'s name in `lang`; English where the display language has no column yet, the code where there is no name at all. */
export function languageName(code: string, lang: string): string {
  return LANGUAGE_NAMES[lang]?.[code] ?? LANGUAGE_NAMES.en[code] ?? code;
}

/** Lower case with the accents gone, so "nhat" finds "Nhật" and "dan mach" finds "Đan Mạch". */
const fold = (text: string) =>
  text
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .replace(/đ/gi, 'd')
    .toLowerCase();

const folded = new Map<string, { code: string; name: string; key: string }[]>();
function all(lang: string) {
  let list = folded.get(lang);
  if (!list) {
    list = Object.keys(LANGUAGE_NAMES.en).map((code) => {
      const name = languageName(code, lang);
      return { code, name, key: fold(name) };
    }).sort((a, b) => a.name.localeCompare(b.name, lang));
    folded.set(lang, list);
  }
  return list;
}

/** The languages whose name (in `lang`) contains `query`, names starting with it first; nothing for an empty query. */
export function searchLanguages(query: string, lang: string, limit = 30): { code: string; name: string }[] {
  const q = fold(query.trim());
  if (!q) return [];
  const hits = all(lang).filter((l) => l.key.includes(q));
  const starts = hits.filter((l) => l.key.startsWith(q));
  return [...starts, ...hits.filter((l) => !l.key.startsWith(q))].slice(0, limit).map(({ code, name }) => ({ code, name }));
}
