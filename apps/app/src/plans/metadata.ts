import type { Metadata } from 'expo-server';

import type { PlanPageData } from './load';

/**
 * A plan page's <head>: the browser tab's title and the card a messenger draws
 * from a pasted link (docs/spec/site.md, "Ways in"): the plan's title and its
 * description. The server writes them into the HTML, so a client that runs no
 * JavaScript reads them. A plan that isn't there is kept out of search engines.
 */
export function planMetadata(data: PlanPageData | null): Metadata {
  const plan = data?.plan;
  if (!plan) return { robots: 'noindex' };
  return {
    title: plan.title,
    description: plan.description,
    openGraph: { type: 'website', siteName: 'Issho', title: plan.title, description: plan.description },
    twitter: { card: 'summary', title: plan.title, description: plan.description },
  };
}
