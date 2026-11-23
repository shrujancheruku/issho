import { setResponseHeaders, type Metadata } from 'expo-server';

import { loadPlan, loadPlans, type PlanList, type PlanPageData } from './load';
import { planMetadata } from './metadata';
import { createServerApi } from './server-api';

/**
 * What the public pages tell a shared cache. The server holds no token and
 * sends the same page to everyone, so it may be kept: a browser for a minute, a
 * CDN for five, and served stale for ten more while it refreshes. A plan that
 * opens, closes or changes shows up within minutes, which is soon enough for a
 * page a person reads before signing up.
 */
export const PUBLIC_PAGE_CACHE = 'public, max-age=60, s-maxage=300, stale-while-revalidate=600';

const NO_STORE = 'no-store';

/**
 * The one rule for what a shared cache may keep: **a page the API failed to
 * fill is never kept.** Expo Router runs a plan page's `generateMetadata` and
 * then its `loader`, and expo-server applies the headers each asked for in the
 * order they were queued, a later `set` replacing an earlier one. So the rule
 * can't rest on who runs last:
 *
 * - a failure sets `no-store` outright, replacing whatever came before;
 * - a success sets the public header only where `no-store` isn't already set.
 *
 * Whichever of the two fails, in whichever order, the page ends `no-store`.
 */
function markFailed(): void {
  setResponseHeaders({ 'Cache-Control': NO_STORE });
}

function markFilled(): void {
  setResponseHeaders((headers) => {
    if (headers.get('Cache-Control') !== NO_STORE) headers.set('Cache-Control', PUBLIC_PAGE_CACHE);
  });
}

/** Marks the page by the one outcome, and hands back what the API said, or nothing for a failure. */
function decide<T>(outcome: Outcome<T>): T | null {
  if (outcome.ok) {
    markFilled();
    return outcome.data;
  }
  markFailed();
  return null;
}

type Outcome<T> = { ok: true; data: T } | { ok: false };

async function attempt<T>(load: () => Promise<T>): Promise<Outcome<T>> {
  try {
    return { ok: true, data: await load() };
  } catch {
    return { ok: false };
  }
}

/**
 * The server's loaders ask in English: it doesn't know the visitor, and a page
 * that varied with who asked couldn't be cached. A reader whose language is
 * another fetches that text after the page loads (plans/use-loaded.ts). A
 * failure hands the screen nothing, so it fetches for itself rather than the
 * server answering with an error page.
 */
export async function plansForServer(): Promise<PlanList | null> {
  return decide(await attempt(() => loadPlans(createServerApi(), 'en')));
}

/**
 * A plan page needs the plan twice in one request, for the head and for the
 * body, and Expo Router runs them one after the other with nothing shared
 * between them. They must read the same answer: a head and a body that came from
 * two calls can disagree about whether the API worked. So both go through one
 * call, kept for a few seconds, failure included, keyed by plan and language.
 *
 * Keeping it is safe because a plan is public and holds nothing per visitor
 * (the server sends no token), so every request for it gets the same answer,
 * and a few seconds is far less than the five minutes the page itself is
 * cached for. A failure is kept the same few seconds so the second of the two
 * reads it too; the next request after that asks again.
 */
const RECENT_MS = 5_000;
const RECENT_MAX = 200;
const recentPlans = new Map<string, { outcome: Promise<Outcome<PlanPageData>>; expires: number }>();

function recentPlan(id: string, lang: 'en'): Promise<Outcome<PlanPageData>> {
  const now = Date.now();
  const key = JSON.stringify([id, lang]);
  const kept = recentPlans.get(key);
  if (kept && kept.expires > now) return kept.outcome;

  for (const [k, v] of recentPlans) if (v.expires <= now) recentPlans.delete(k);
  if (recentPlans.size >= RECENT_MAX) recentPlans.clear();

  // It counts as in flight until it settles, so a slow call is not asked twice either.
  const entry = { outcome: attempt(() => loadPlan(createServerApi(), id, lang)), expires: Number.POSITIVE_INFINITY };
  recentPlans.set(key, entry);
  void entry.outcome.then(() => {
    entry.expires = Date.now() + RECENT_MS;
  });
  return entry.outcome;
}

export async function planForServer(id: string): Promise<PlanPageData | null> {
  return decide(await recentPlan(id, 'en'));
}

/**
 * The same page's head, for the route's `generateMetadata`. It reads the call
 * the loader reads (see `recentPlan`), so it sees the same success or failure,
 * and asks for the same header. A head with no title is a page the API failed to
 * fill: `no-store`.
 */
export async function planMetadataForServer(id: string): Promise<Metadata> {
  return planMetadata(decide(await recentPlan(id, 'en')));
}
