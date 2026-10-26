# Marketing site

Static landing page for Issho, published to GitHub Pages by `.github/workflows/pages.yml` on every push to `main` that touches `site/`.

- `index.html`: single self-contained page (inline CSS, no build step).
- `logo.svg` is a copy of `assets/brand/logo.svg`; `favicon.svg` is `mark.svg` on a solid `#111111` circle. Rebuild them if the brand assets change.
- Dark theme: the design system's "Ai" indigo night ([docs/design-system.md](../docs/design-system.md)), with gold `#C9A227` for the thread and the button. Red `#BC002D` appears only as the logo's sun (the logo itself and the 日 card). Headings and statement text use Dela Gothic One; body is Zen Kaku Gothic New (both Google Fonts).
- The email/city form is a placeholder: it prevents submission and sends nothing. Wire it to a backend before collecting real data, and update the privacy wording at the same time.

Preview locally: `python3 -m http.server -d site 8000`.

## Design notes

- Section dividers are a seigaiha (wave) pattern: one `<symbol id="seigaiha">` at the top of `<body>`, reused by each `.divider` with `--from`/`--to` set to the colours of the sections either side.
- The community steps are a gold thread with a gold knot at each step, drawn in CSS (no images).
- The kanji cards colour each part like the logo: gold strokes (一, 糸), black ink outlined in gold (者), red sun (日).
- Kanji in the breakdown use Noto Serif JP ExtraBold (800), the same font and weight as the logo, loaded as a four-character subset from Google Fonts.

**Hosting:** a static-assets Worker (`issho-site`, `wrangler.jsonc`) at `joinissho.com` and `www.joinissho.com`; `npx wrangler deploy` in this folder publishes it (`.assetsignore` keeps this README and the config out).
