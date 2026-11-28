import type { StringKey } from '@/i18n/strings';

/** What the profile's questions collect, as the form holds it. */
export type Answers = {
  name: string;
  dateOfBirth: string;
  gender: Gender | '';
  languages: string[];
  /** `null` until the person answers: either answer is accepted, but one is required. */
  livesInCity: boolean | null;
  acceptedCodeOfConduct: boolean;
  onlyWomenAndNonBinary: boolean;
};

export const GENDERS = [
  { value: 'woman', labelKey: 'profile.gender.woman' },
  { value: 'man', labelKey: 'profile.gender.man' },
  { value: 'non-binary', labelKey: 'profile.gender.nonBinary' },
  { value: 'prefer-not-to-say', labelKey: 'profile.gender.preferNot' },
] as const satisfies readonly { value: string; labelKey: StringKey }[];
export type Gender = (typeof GENDERS)[number]['value'];

/** Who is offered "only women and non-binary people" (docs/spec/people.md). */
export const offersWomenOnly = (gender: Answers['gender']) => gender === 'woman' || gender === 'non-binary';

/** The languages listed to tick; the API takes any language code, and these are the ones people here speak most. */
export const LANGUAGES = [
  { code: 'en', labelKey: 'language.en' },
  { code: 'vi', labelKey: 'language.vi' },
  { code: 'ja', labelKey: 'language.ja' },
  { code: 'ko', labelKey: 'language.ko' },
  { code: 'zh', labelKey: 'language.zh' },
  { code: 'fr', labelKey: 'language.fr' },
  { code: 'de', labelKey: 'language.de' },
  { code: 'es', labelKey: 'language.es' },
  { code: 'pt-BR', labelKey: 'language.pt-BR' },
  { code: 'ru', labelKey: 'language.ru' },
  { code: 'th', labelKey: 'language.th' },
  { code: 'id', labelKey: 'language.id' },
  { code: 'it', labelKey: 'language.it' },
  { code: 'hi', labelKey: 'language.hi' },
] as const satisfies readonly { code: string; labelKey: StringKey }[];

export const NAME_MAX = 40;
export const ADULT_AGE = 18;

export const validName = (name: string) => name.trim().length >= 1 && [...name.trim()].length <= NAME_MAX;

const isLeap = (year: number) => (year % 4 === 0 && year % 100 !== 0) || year % 400 === 0;
const pad = (n: number) => String(n).padStart(2, '0');

/** `today` as "YYYY-MM-DD" on the device's calendar. The API checks again, by the city's. */
export function localDay(now: Date = new Date()): string {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

/** Whether `dob` is a real calendar date, "YYYY-MM-DD", in the past. */
export function isRealPastDate(dob: string, today: string): boolean {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dob);
  if (!m) return false;
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])];
  const real = new Date(Date.UTC(y, mo - 1, d));
  const exists = real.getUTCFullYear() === y && real.getUTCMonth() === mo - 1 && real.getUTCDate() === d;
  return exists && y >= 1900 && dob <= today;
}

/**
 * Whether someone born on `dob` is 18 on `today`. A 29 February birthday turns
 * 18 on 1 March in a common year (the conservative reading, as the API does).
 */
export function isAdult(dob: string, today: string): boolean {
  const [y, mo, d] = dob.split('-').map(Number);
  const year = y + ADULT_AGE;
  const birthday = mo === 2 && d === 29 && !isLeap(year) ? `${year}-03-01` : `${year}-${pad(mo)}-${pad(d)}`;
  return birthday <= today;
}
