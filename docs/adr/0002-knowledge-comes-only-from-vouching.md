# Issho's knowledge comes only from vouching

Issho stores a fact about a place only when the owner or an insider has **vouched** for it, in person for an activity, a meeting point or a stretch. What is stored is the **piece**, the facts recorded with it, the date and a Google place ID. There is no crawl, no catalogue of places and nothing derived from Google content. We chose this for two reasons. A [prototype of twelve plans](../spec/plan-bar.md#what-the-prototype-showed) showed that plans built on what a local person knows scored 4.3 on "would you send a friend?", against 2.3 for Google alone. And [the terms of Google and the review sites](https://github.com/shrujancheruku/issho/issues/537) forbid storing or deriving from their content, which v1's catalogue did. The rules are in [Plan knowledge](../spec/plan-knowledge.md).

The cost is reach: a city opens only where a person can visit, on 12 standing plans, and knowledge grows at the speed people vouch. We accept that, because a plan that is wrong on the day costs more than a plan that doesn't exist.

## What follows from it

Each of these looks like a gap to fill. Each is deliberate.

- **No place data is copied in.** Search, local lists and people's suggestions give **leads** only: our own note, links and a place ID, kept apart on a leads list and never shown to a member or used in a plan.
- **A model vouches for nothing** and never names a place from memory. It finds leads, sketches outlines, drafts from vouched knowledge and judges plans.
- **Google's grounding is never used for the lead search** (Google Search, Web Grounding for Enterprise, Maps): its terms forbid storing the results or building an index from its links. The lead search uses a web search tool whose results we may keep notes on.
- **Google gives identity only**: the place ID, the map pin, live hours shown beside ours and the closed flag. Its content is shown live with attribution and never stored, cached or used to compute anything we keep.
- **No geometry.** An area is a label the voucher picks, with no boundary stored; there is no point-in-polygon, no PostGIS and no router of our own. A pleasant route is a stretch someone walked, and Google's live travel time only checks the minutes the maker stated.

## Considered options

- **A catalogue built from Google Places, ranked by a model** (v1). Rejected: it breaks Google's terms as soon as anything derived is stored, and it produced plans the owner would not send a friend on.
- **Our own per-street walking score on a self-hosted router.** Rejected: OpenStreetMap says almost nothing about pleasantness in Da Nang–Hoi An, and the work is large for what a walked stretch gives for free.
- **Scraping where locals actually post** (TikTok, Facebook, Zalo). Rejected: no API, and against their terms.
