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
  setResponseHeaders({ 'Cache-Control': PUBLIC_PAGE_CACHE });
  return loadPlans(createServerApi(), 'en').catch(() => null);
}

export async function planForServer(id: string): Promise<PlanPageData | null> {
  setResponseHeaders({ 'Cache-Control': PUBLIC_PAGE_CACHE });
  return loadPlan(createServerApi(), id, 'en').catch(() => null);
}

/** The same page's head, for the route's `generateMetadata`. */
export async function planMetadataForServer(id: string): Promise<Metadata> {
  return planMetadata(await loadPlan(createServerApi(), id, 'en').catch(() => null));
}
