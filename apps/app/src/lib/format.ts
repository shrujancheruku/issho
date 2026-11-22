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

/**
 * A price in whole units of the plan's currency, in the display language's
 * conventions. Prices are never converted (proto/issho/v1/plan.proto), and
 * whole units: a dong has no minor unit worth showing.
 */
export const formatPrice = (amount: number, currency: string, lang: Language) =>
  new Intl.NumberFormat(lang, { style: 'currency', currency, maximumFractionDigits: 0 }).format(amount);

/** A plan's start and end, "HH:MM" on the city's clock, as one range. They are already the city's time, so nothing is converted. */
export const formatTimeRange = (startsAt: string, endsAt: string) => `${startsAt}–${endsAt}`;

/**
 * A calendar date ("YYYY-MM-DD", the city's own, as ticks and hear-by dates
 * are) written like any other date. A date has no instant, so it is read at
 * noon UTC and written in UTC: the city's zone cannot move it to another day.
 */
export const formatCalendarDate = (date: string, lang: Language) => formatDate(new Date(`${date}T12:00:00Z`), { timeZone: 'UTC', lang });
