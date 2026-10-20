# Marketing site

Static landing page for Issho, published to GitHub Pages by `.github/workflows/pages.yml` on every push to `main` that touches `site/`.

- `index.html`: single self-contained page (inline CSS, no build step).
- `logo.svg`, `favicon.svg`: copies of `assets/brand/logo.svg` and `mark.svg`. Re-copy them if the brand assets change.
- Palette comes from the brand: red `#BC002D`, gold `#C9A227`, black `#111111`.
- The email/city form is a placeholder: it prevents submission and sends nothing. Wire it to a backend before collecting real data, and update the privacy wording at the same time.

Preview locally: `python3 -m http.server -d site 8000`.
