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

/**
 * The city's calendar date as a "YYYY-MM-DD" key, the form ticks, hear-by dates
 * and a plan's round end travel in (never shown: `formatCalendarDate` writes
 * it for people). It is the date on the city's clock, whatever the device's
 * zone. en-CA is used for its year-month-day order only, as a key and not as
 * text in the display language, so it takes no `lang`.
 */
export const cityDateKey = (date: Date, timeZone: string) =>
  new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(date);

/** The device's own time zone, which only the outing's "beside it" line reads. */
export const deviceTimeZone = () => Intl.DateTimeFormat().resolvedOptions().timeZone;

/**
 * The instant a wall-clock time ("YYYY-MM-DD", "HH:MM") is in a zone. A zone's
 * offset is read at the guess and again at the corrected instant, which settles
 * it on either side of a daylight-saving change.
 */
function instantIn(date: string, time: string, timeZone: string): Date {
  const wall = Date.parse(`${date}T${time}:00Z`);
  const offsetAt = (at: number) => {
    const parts = Object.fromEntries(
      new Intl.DateTimeFormat('en-US', { timeZone, hourCycle: 'h23', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
        .formatToParts(new Date(at))
        .map((p) => [p.type, p.value]),
    );
    return Date.UTC(+parts.year, +parts.month - 1, +parts.day, +parts.hour, +parts.minute, +parts.second) - at;
  };
  const first = wall - offsetAt(wall);
  return new Date(wall - offsetAt(first));
}

/**
 * When an outing is, on the city's clock, and on the device's clock when that
 * reads differently (docs/spec/outings.md: "with the device's time beside it
 * when different"). An outing's date and start are the city's own wall time, so
 * the device's reading is found from the instant they name. `device` is null
 * when the device reads the same.
 */
export function formatOutingWhen({ date, startsAt, timeZone, lang, deviceZone = deviceTimeZone() }: { date: string; startsAt: string; timeZone: string; lang: Language; deviceZone?: string }) {
  const at = instantIn(date, startsAt, timeZone);
  const city = formatDateTime(at, { timeZone, lang });
  const device = formatDateTime(at, { timeZone: deviceZone, lang });
  return { city, device: device === city ? null : device };
}
