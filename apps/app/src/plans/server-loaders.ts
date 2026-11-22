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

/**
 * The server's loaders ask in English: it doesn't know the visitor, and a page
 * that varied with who asked couldn't be cached. A reader whose language is
 * another fetches that text after the page loads (plans/use-loaded.ts). A
 * failure hands the screen nothing, so it fetches for itself rather than the
 * server answering with an error page.
 */
export async function plansForServer(): Promise<PlanList | null> {
  return cacheIfFilled(loadPlans(createServerApi(), 'en'));
}

export async function planForServer(id: string): Promise<PlanPageData | null> {
  return cacheIfFilled(loadPlan(createServerApi(), id, 'en'));
}

/**
 * The public header goes on only once the API has answered. A failure leaves
 * an empty page the client refills, and a shared cache must not keep that for
 * minutes, so it says `no-store`.
 */
async function cacheIfFilled<T>(pending: Promise<T>): Promise<T | null> {
  try {
    const data = await pending;
    setResponseHeaders({ 'Cache-Control': PUBLIC_PAGE_CACHE });
    return data;
  } catch {
    setResponseHeaders({ 'Cache-Control': 'no-store' });
    return null;
  }
}

/**
 * The same page's head, for the route's `generateMetadata`. It is a second call
 * to the API, so it can fail after the body loaded; a head with no title is a
 * page the API failed to fill, and the same rule holds: `no-store`.
 */
export async function planMetadataForServer(id: string): Promise<Metadata> {
  try {
    return planMetadata(await loadPlan(createServerApi(), id, 'en'));
  } catch {
    setResponseHeaders({ 'Cache-Control': 'no-store' });
    return planMetadata(null);
  }
}
