import { aPlan } from '@/test-support/plan-fixtures';

import { planMetadata } from '../metadata';

describe("a plan page's head", () => {
  it("carries the plan's title and description for the tab and for a link card", () => {
    const meta = planMetadata({ lang: 'en', plan: aPlan() });
    expect(meta).toMatchObject({
      title: 'Lantern making in Hoi An',
      description: 'Da Nang–Hoi An · Fold and paint a silk lantern with a local maker.',
      openGraph: { title: 'Lantern making in Hoi An', description: 'Da Nang–Hoi An · Fold and paint a silk lantern with a local maker.', siteName: 'Issho' },
      twitter: { card: 'summary', title: 'Lantern making in Hoi An', description: 'Da Nang–Hoi An · Fold and paint a silk lantern with a local maker.' },
    });
  });

  it('keeps a plan that is not there out of search engines', () => {
    expect(planMetadata({ lang: 'en', plan: null })).toEqual({ robots: 'noindex' });
    expect(planMetadata(null)).toEqual({ robots: 'noindex' });
  });
});
