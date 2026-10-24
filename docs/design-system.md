# Design system

How Issho looks across the app, the admin site and the marketing page. The full brand book, with live component previews, is the [Issho design system](https://claude.ai/artifact/4NGW8kKPbQ6QvWKfmvjpTv) artifact (private to the owner). This page records what the code follows.

The logo spells 一緒 as one gold thread joining people under one red sun. The screens follow that picture:

- **Indigo cloth** ("Ai", 藍, the traditional dye for thread) as the ground.
- **The gold thread** joins things and marks the one thing to do.
- **The red sun lives only in the logo.** Screens never use red for fills, icons or decoration; the one exception is error text.

## Where the values live

| Surface | File |
|---|---|
| App (iOS, Android, web) | `apps/mobile/src/constants/theme.ts`: `Colors` (per theme), `Brand` (fixed), `BodyFont`, `DisplayFont` |
| Admin site | `apps/admin/src/styles.css`, the `:root` block |
| Marketing page | `site/index.html`, the `:root` block (dark only) |

Keep the three in step when a value changes.

## Colour

| Role | Light | Dark | Use |
|---|---|---|---|
| ground | `#F3F5F9` | `#0D1330` | Screen background (`background` in the app) |
| surface | `#FFFFFF` | `#161E40` | Cards and sheets |
| surface-sunk | `#E7EBF3` | `#222B52` | Inputs, unselected chips, chat bubbles (`backgroundElement`) |
| line | `#D5DBE7` | `#2C3765` | Hairlines, empty pills (`backgroundSelected`) |
| line-strong | `#7A849C` | `#6C78AA` | Input and outlined-control borders (3:1) |
| text | `#111111` | `#F1F3F8` | Body text |
| text-muted | `#545D73` | `#A9B1CB` | Secondary text (`textSecondary`) |
| accent | `#24307A` | `#3A4A9E` | Selection, the active tab, signed-up marks, your chat bubbles; white text on it |
| link | `#24307A` | `#9FB0FF` | Links, and the accent wherever it is text |
| gold-text | `#7D5E0A` | `#C9A227` | Gold as text (gold itself is 2.2:1 on light grounds) |
| error | `#BC002D` | `#FF7A90` | Error messages, and dangers in the admin |
| ok | `#2F6B3D` | `#7FC28E` | Success, always with a word |

Fixed in both themes (`Brand`): gold `#C9A227` (the main button, routes, timelines; ink text on it), gold tint `#F2E6BD`, deep gold `#7D5E0A` (white text on it), ink `#111111`, and red `#BC002D` (logo only).

**Interest levels** are one gold family, pale to deep, so they read by lightness as well as hue: open to try `goldTint`, like `gold`, love `goldDeep`. Map pins use the same colours.

Every text pair above is at least 4.5:1 on ground, surface and surface-sunk in its theme.

## Type

- **Display:** Dela Gothic One, the wordmark's face, for titles and plan or crew names. One weight: never set a font weight on it.
- **Body:** Zen Kaku Gothic New (400, 500, 700), which sets Japanese and Latin with the same rhythm. In the app each weight is its own family, and `ThemedText` picks it from the style's `fontWeight`. Inputs set `BodyFont.regular` themselves.
- The admin bundles both fonts with `@fontsource` (its CSP allows only `'self'`) and loads only 400 and 700. The marketing page loads them from Google Fonts.

## Switching the ground later

Ai was chosen on 2026-10-25, with three alternatives kept open: Sora 空 (sky), Yūyake 夕焼け (dusk) and Koke 苔 (moss). Their exact values are in the design system artifact under "Alternative grounds". A switch changes only ground, surface, surface-sunk, line, text-muted, accent, and the dark text and link, in the three files above. Recheck line-strong for 3:1 afterwards. Components refer to roles, never to "indigo", so nothing else changes.
