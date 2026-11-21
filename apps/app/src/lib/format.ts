import type { Language } from '@/i18n/strings';

/**
 * Every date and time the app shows goes through here. A time is written on the
 * city's clock, never the device's, so a visitor abroad reads the hour the
 * outing happens, and in the display language's conventions
 * (docs/spec/languages.md, "Names and formats"). The time zone is required:
 * there is no default to fall back to by accident.
 */
export interface FormatOptions {
  /** An IANA zone name, the city's (for Da Nang–Hoi An, Asia/Ho_Chi_Minh). */
  timeZone: string;
  lang: Language;
}

const DATE: Intl.DateTimeFormatOptions = { weekday: 'short', month: 'short', day: 'numeric' };
const TIME: Intl.DateTimeFormatOptions = { hour: 'numeric', minute: '2-digit' };

const format = (date: Date, { timeZone, lang }: FormatOptions, style: Intl.DateTimeFormatOptions) =>
  new Intl.DateTimeFormat(lang, { ...style, timeZone }).format(date);

export const formatDate = (date: Date, opts: FormatOptions) => format(date, opts, DATE);
export const formatTime = (date: Date, opts: FormatOptions) => format(date, opts, TIME);
export const formatDateTime = (date: Date, opts: FormatOptions) => format(date, opts, { ...DATE, ...TIME });
