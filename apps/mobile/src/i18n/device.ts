import { baseLanguage } from './strings';

/**
 * ISO 639-1 codes: the only language codes the server accepts. A Go test
 * (backend/internal/platform/i18n) checks this list matches the server's.
 */
const languageCodes = new Set(
  `aa ab ae af ak am an ar as av ay az ba be bg bi bm bn bo br bs ca ce ch co
cr cs cu cv cy da de dv dz ee el en eo es et eu fa ff fi fj fo fr fy ga gd gl gn gu gv ha he hi ho hr
ht hu hy hz ia id ie ig ii ik io is it iu ja jv ka kg ki kj kk kl km kn ko kr ks ku kv kw ky la lb lg
li ln lo lt lu lv mg mh mi mk ml mn mr ms mt my na nb nd ne ng nl nn no nr nv ny oc oj om or os pa pi
pl ps pt qu rm rn ro ru rw sa sc sd se sg si sk sl sm sn so sq sr ss st su sv sw ta te tg th ti tk tl
tn to tr ts tt tw ty ug uk ur uz ve vi vo wa wo xh yi yo za zh zu`.split(/\s+/),
);

/**
 * Chinese in Traditional characters, the one display language that isn't an
 * ISO 639-1 code (backend i18n.TraditionalChinese); "zh" is Simplified.
 */
const TraditionalChinese = 'zh-Hant';

/** Whether a Chinese tag is written in Traditional characters: zh-Hant-*, or Taiwan, Hong Kong, Macau. */
function traditional(tag: string | undefined): boolean {
  const parts = (tag ?? '').split(/[-_]/).slice(1).map((p) => p.toLowerCase());
  if (parts.includes('hant')) return true;
  if (parts.includes('hans')) return false;
  return parts.some((p) => p === 'tw' || p === 'hk' || p === 'mo');
}

/**
 * Device codes with no ISO 639-1 code of their own, mapped to the closest one
 * the server has. Filipino is standardised Tagalog; Cantonese is written in
 * Chinese characters; iw, in and ji are the old codes some systems still
 * report for Hebrew, Indonesian and Yiddish.
 */
const aliases: Record<string, string> = { fil: 'tl', yue: 'zh', iw: 'he', in: 'id', ji: 'yi' };

/**
 * The display languages to start with, from the device's preferred languages
 * (most preferred first): the first two the server supports. Languages it
 * can't show (e.g. Hawaiian, "haw") are skipped rather than used and rejected
 * at the end of onboarding; with none left, English.
 */
export function deviceLanguages(locales: readonly { languageCode?: string | null; languageTag?: string }[]): {
  primary: string;
  secondary: string;
} {
  const codes: string[] = [];
  for (const l of locales) {
    let code = baseLanguage(l.languageCode ?? l.languageTag);
    code = aliases[code] ?? code;
    if (code === 'zh' && traditional(l.languageTag)) code = TraditionalChinese;
    if ((languageCodes.has(code) || code === TraditionalChinese) && !codes.includes(code)) codes.push(code);
  }
  return { primary: codes[0] ?? 'en', secondary: codes[1] ?? '' };
}
