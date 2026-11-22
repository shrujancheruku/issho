import type { Metadata } from 'expo-server';

import { DEFAULT_CITY_NAME } from './city';
import type { PlanPageData } from './load';

/**
 * A plan page's <head>: the browser tab's title and the card a messenger draws
 * from a pasted link (docs/spec/site.md, "Ways in"): the plan's title, the city
 * and its one-line summary. The server writes them into the HTML, so a client that runs no
 * JavaScript reads them. A plan that isn't there is kept out of search engines.
 */
export function planMetadata(data: PlanPageData | null): Metadata {
  const plan = data?.plan;
  if (!plan) return { robots: 'noindex' };
  const description = `${DEFAULT_CITY_NAME} · ${plan.description}`;
  return {
    title: plan.title,
    description,
    openGraph: { type: 'website', siteName: 'Issho', title: plan.title, description },
    twitter: { card: 'summary', title: plan.title, description },
  };
}
