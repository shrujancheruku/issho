# The site

The public website at joinissho.com. Sources are cited as in the [README](README.md). The code is `site/` (one HTML file) and `site/README.md`.

## What v1 decided

| Decision | Source |
|---|---|
| The site is **a single static HTML file**, with no build step, kept separate from the app. | Log, 2026-10-20 |
| It **tells one outing along a gold thread**: a plan → date set → going → on the day → how was it → a crew. "Shows what Issho does instead of describing it." Each stage has a picture of the app's own screen in the app's own wording. | Log, 2026-10-26 |
| The headline is **"Find your people through the things you already love."** The stage headings: "Pick something you'd do anyway." / "Enough of you said yes. It's on." / "A few people, not a crowd." / "Show up. The activity does the talking." / "Pick who you'd meet again. Privately." / "Strangers on Saturday, regulars by spring." | `site/index.html` |
| A section explains the name: "Issho (一緒) means 'together'", with the logo's kanji. | `site/index.html` |
| The **"Can you go?" card seats the group round a table** and keeps who said yes hidden until you confirm, as the app does. | Log, 2026-10-27 |
| The page's only action is **"Join the waitlist"**: an email and a city. The line under it: "We only use your email to tell you when Issho opens near you." | `site/index.html` |
| It uses the app's colours, fonts and motion, in light and dark, and took the gold palette before the app did. | Log, 2026-10-26; `site/README.md` |
| It is hosted on Cloudflare and deploys from CI when `site/` changes. | Log, 2026-10-27 |
| The site is **English only** (with Japanese in the name section and the footer's goodbye). | `site/index.html` |
| The README's animated panels are generated from the site's fragments. | Log, 2026-10-27 |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **The site leads with Da Nang–Hoi An.** Remove "Tokyo first, then your city." and any other Tokyo-first copy. | No | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| **The waitlist signup goes live during the Pilot.** It collects an email and a city only: "the first entries of the waitlist". Stored through a public API call in our own database and listed in the admin; no third-party form service. Before it goes live: a consent tick box for that one purpose linked to the privacy policy, a way to be removed, rate limiting and a bot check (Cloudflare Turnstile). The owner may invite people from the list by hand. | No | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417); ADR 0002 (verdicts) |
| **Legal pages on the site**: privacy policy, terms, and a page for requesting account deletion (Play requires one). In Vietnamese and English, Vietnamese binding. | No | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412); [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362) |
| **`joinissho.com/pilot`**: one QR code per invite points here. It sends an iPhone to the TestFlight link, Android to the Google Group then Play, anything else to the web app. | No | [Distribution for each platform](https://github.com/shrujancheruku/issho/issues/343) |
| The site gets a smoke test for its form. | No | ADR 0002 (verdicts) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| GitHub Pages | A Cloudflare Worker at joinissho.com | One workflow for all three sites; the old copy had gone stale | Log, 2026-10-27 |
| A row of initials and a button for "Can you go?" | The group round a table | "The old card showed nothing happening" | Log, 2026-10-27 |
| Tokyo first | Da Nang–Hoi An first (decided, unbuilt) | The first city changed | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| A form that deliberately does nothing "until a backend and privacy wording exist" | A live waitlist with consent (decided, unbuilt) | The Pilot's scope | Same |

## Left open, parked, cut or only planned

- **The waitlist form sends nothing.** It prevents submission. No email has ever been collected through it.
- **There are no legal pages.** The site has one page, and nothing in the repo links a privacy policy or terms.
- **No Vietnamese version** of the site.
- **What someone on the waitlist is told and when**, and how the site's entries join an in-app waitlist, were left to [Waitlist and unlock](https://github.com/shrujancheruku/issho/issues/339), which was deferred.
- **Google Fonts** are loaded from Google; self-hosting them or disclosing it in the privacy policy was raised by the audit and not decided.
- **No analytics** on the site.
- **Whether the site should do more than explain and collect emails** (list plans, show a city, let someone sign in) was never asked. The web app is a separate address, `app.joinissho.com`.

## Where the code and the documents disagree

- **"Tokyo first, then your city."** is the live heading of the waitlist section. The decision is Da Nang–Hoi An first.
- **The plan card's map is Kichijoji, Tokyo**: a walk from the station round Inokashira Pond to a café.
- **"The four of you"**: the crew scene shows four people on a thread; a crew is three or more.
- **The crew scene shows a crew being offered by Issho**; the Pilot decision is that the owner offers them. The site's wording ("You got on well") still fits either.
- `docs/design-system.md` says the marketing page's colours are "dark only"; `site/README.md` and the page have light and dark with a switch.
- `site/README.md` matches the page, including the placeholder form.
