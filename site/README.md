# Marketing site

Static landing page for Issho, published to GitHub Pages by `.github/workflows/pages.yml` on every push to `main` that touches `site/`.

- `index.html`: single self-contained page (inline CSS, no build step).
- `logo.svg`, `favicon.svg` (the round blue mark), `apple-touch-icon.png` and `og-image.png` (the link preview, 1200×630) are generated from `assets/brand/` by `scripts/brand/build_variants.py` (`make app-icons`). Don't edit them by hand.
- Light and dark, following the system setting like the app, with a switch in the header (in place of a second waitlist link) that overrides it and is remembered in `localStorage`. A tiny script in `<head>` applies the remembered choice before the first paint, so the page never flashes the other theme. Switching cross-fades the colours (the `theming` class turns on colour transitions for half a second; instant with reduced motion). Without JavaScript the switch stays hidden and the page follows the system. The themes are the app's two canvases ([docs/design-system.md](../docs/design-system.md)), Ai night with the Kogane gold `#DEA92C` and its tint `#F2E2B2` for the big headings, and the light canvas `#F3F5F9` with ink headings and deep gold `#885716` for gold text. Gold fills the thread and the button in both. The three interest chips are filled as in the app (vermilion "Love it", gold "Like it", pale gold "Open to try"); shu red `#DA4528` is only the 日 sun. Headings use Dela Gothic One; body and Japanese use Zen Kaku Gothic New (both Google Fonts).
- The email/city form is a placeholder: it prevents submission and sends nothing. Wire it to a backend before collecting real data, and update the privacy wording at the same time.

Preview locally: `python3 -m http.server -d site 8000`.

## Design notes

- The page tells one outing from start to finish: a plan, date set, going, on the day, how was it, then a crew. A gold thread fades in just below the header logo and runs down the left, with a knot at each stage; the crew is a closed loop where the thread ends.
- A small script at the end of the page draws the thread as you scroll (the fill ends at the middle of the screen, and each knot turns gold when the fill reaches it). It uses scroll events, not CSS scroll-driven animations, so it works in every browser. Without JavaScript, or with reduced motion, the whole thread is drawn. The fill ends at the bottom of `.story`, which is why the crew stage sits just after it.
- Each stage has a fragment of the app's screens in the app's own wording and look (`apps/mobile/src/i18n/en.json` and `human/en.json`, the app's date format, plan and crew names in the display face). They are pictures, so nothing in them is focusable. The typing dots stop with reduced motion.
- The kanji in "the name" are coloured like the logo (gold strokes, ink outlined in gold for 者, red sun) and set in Noto Serif JP ExtraBold, the logo's font, loaded as a subset from Google Fonts.

**Hosting:** a static-assets Worker (`issho-site`, `wrangler.jsonc`) at `joinissho.com` and `www.joinissho.com`; `npx wrangler deploy` in this folder publishes it (`.assetsignore` keeps this README and the config out).
