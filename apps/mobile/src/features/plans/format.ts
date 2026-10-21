import type { FeedPlan } from '@issho/api-client';

/** "Sat, Jan 5": a city date ("2030-01-05") in the given language. */
export function formatDate(date: string, lang: string) {
  const [y, m, d] = date.split('-').map(Number);
  const opts: Intl.DateTimeFormatOptions = { weekday: 'short', month: 'short', day: 'numeric', timeZone: 'UTC' };
  const at = new Date(Date.UTC(y, m - 1, d));
  try {
    return new Intl.DateTimeFormat(lang, opts).format(at);
  } catch {
    return new Intl.DateTimeFormat('en', opts).format(at);
  }
}

/**
 * The last day to sign up. Deadlines fall at the start of a day (local
 * midnight), so "until Nov 1, 12:00 AM" would read as if Nov 1 still
 * counted: show the day before instead. In the device's time zone, which is
 * the city's for people using the app there.
 */
export function lastSignupDay(ts: FeedPlan['signupDeadline'], lang: string) {
  if (!ts) return '';
  const at = new Date(Number(ts.seconds) * 1000 - 1);
  const opts: Intl.DateTimeFormatOptions = { weekday: 'short', month: 'short', day: 'numeric' };
  try {
    return new Intl.DateTimeFormat(lang, opts).format(at);
  } catch {
    return new Intl.DateTimeFormat('en', opts).format(at);
  }
}

/**
 * An activity name without the examples in brackets that help on the
 * onboarding tiles ("Food tour (ramen, desserts, markets …)"): in a timeline
 * the venue already says which. Handles Japanese and Chinese full-width
 * brackets too. Mirrors withoutExamples in the API's plan titles.
 */
export function withoutExamples(name: string) {
  return name.replace(/\s*[(（][^()（）]*[)）]\s*$/, '');
}
