import { Code, ConnectError } from '@connectrpc/connect';
import type { Api, Plan } from '@issho/api-client';

import type { Language } from '@/i18n/strings';

import { DEFAULT_CITY } from './city';

/**
 * What the Plans screens read, from the generated client. The server's loaders
 * (a route's `loader`, on the web) and the client's own fetch both go through
 * here, so a page reads the same wherever it was drawn. `lang` is what was
 * asked for: the plan's own `language` says what came back, which is English
 * when it has no text in the one asked for.
 */
export type PlanList = { lang: Language; plans: Plan[] };
export type PlanPageData = { lang: Language; plan: Plan } | { lang: Language; plan: null };

export async function loadPlans(api: Pick<Api, 'plans'>, lang: Language): Promise<PlanList> {
  const { plans } = await api.plans.listPlans({ city: DEFAULT_CITY, displayLanguage: lang });
  return { lang, plans };
}

/** A plan that is missing (or not open yet, or in a closed city: the API says the same for all) is `plan: null`; any other failure throws. */
export async function loadPlan(api: Pick<Api, 'plans'>, id: string, lang: Language): Promise<PlanPageData> {
  try {
    const { plan } = await api.plans.getPlan({ id, displayLanguage: lang });
    return plan ? { lang, plan } : { lang, plan: null };
  } catch (err) {
    if (ConnectError.from(err).code === Code.NotFound) return { lang, plan: null };
    throw err;
  }
}
