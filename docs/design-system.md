# Design system

How Issho looks across the app, the admin site and the marketing page. The full brand book, with live component previews, is the [Issho design system](https://claude.ai/artifact/4NGW8kKPbQ6QvWKfmvjpTv) artifact (private to the owner). This page records what the code follows.

The logo spells 一緒 as one gold thread joining people under one red sun. The screens follow that picture:

- **Indigo cloth** ("Ai", 藍, the traditional dye for thread) as the ground.
- **The gold thread** (Kogane, 黄金) joins things and marks the one thing to do. Every outing shows it as five knots: signed up, date set, going, on the day, how was it.
- **The red sun lives only in the logo.** Screens never use red for fills, icons or decoration; the exceptions are error text and the vermilion "I love it" mark (below).

## Where the values live

| Surface | File |
|---|---|
| App (iOS, Android, web) | `apps/mobile/src/constants/theme.ts`: `Colors` (per theme), `Brand` (fixed), `BodyFont`, `DisplayFont`; motion in `constants/motion.ts` |
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
| text | `#131B40` | `#F1F3F8` | Body text (Ai-black: indigo so dark it reads as black) |
| text-muted | `#545D73` | `#A9B1CB` | Secondary text (`textSecondary`) |
| accent | `#24307A` | `#3A4A9E` | Selection, the active tab, signed-up marks, your chat bubbles; white text on it |
| link | `#24307A` | `#9FB0FF` | Links, and the accent wherever it is text |
| gold-text | `#885716` | `#DEA92C` | Gold as text (gold itself is 2.1:1 on light grounds) |
| love | `#CC3719` | `#EB603A` | "I love it" only: interest chips, map pins, the Plans section mark. White text on it in light, ink in dark |
| error | `#BC002D` | `#FF7A90` | Error messages, and dangers in the admin |
| ok | `#2F6B3D` | `#7FC28E` | Success, always with a word |

Fixed in both themes (`Brand`): gold `#DEA92C` (the main button, the thread, routes; ink text on it), gold tint `#F2E2B2`, deep gold `#885716` (white text on it), ink `#131B40`, and the shu sun `#DA4528` (logo and notification icon only). The gold was chosen on 2026-10-26 ("Kogane"), replacing `#C9A227`.

**Interest levels** read by lightness as well as hue: open to try `goldTint`, like `gold`, love `love` (vermilion). Map pins and the Plans sections use the same colours. Urgency and deadlines stay gold, never vermilion.

Every text pair above is at least 4.5:1 on ground, surface and surface-sunk in its theme.

## Light and dark

The app follows the phone's setting by default; You → Appearance overrides it with Light or Dark (`features/appearance`). The choice is stored on the device and applied before the first screen draws, and native controls follow it through `Appearance.setColorScheme`. The admin follows the system only.

## Motion

Timings live in `apps/mobile/src/constants/motion.ts` and reach components through `useMotion()`; components never write their own numbers.

| Token | Value | Use |
|---|---|---|
| press | 100 ms, scale 0.97 | Every tappable thing (`PressableScale`) |
| change | 180 ms | Colour and label swaps; a button becoming its result (`ResultButton`) |
| move | 280 ms | Things entering a list (new chat messages), a countdown rolling |
| draw | 450 ms a segment | The thread growing to a new stage (once per outing per session) |
| gather | spring, damping 12, stiffness 180 | Knots tying, a crew starting |
| settle | spring, damping 20, stiffness 200 | Sheets and drags: no overshoot |
| breathe | 2.4 s loop | The one live thing on a screen (typing dots run at twice its pace) |
| pinStagger | 45 ms | Map pins dropping in one after another, the first time a map opens this session (at most 12 steps) |
| routeDraw | 1.6 s | A plan's route drawing itself, whatever its length, once per route per session; stops pop in as the line reaches them |
| walk, walkRest | 5.2 s, 1.8 s | The gold walker strolling a drawn route, then resting; only while the screen shows, never with reduced motion |

The rules: loops only on something live, and one per screen, paused when the screen is hidden or the app is in the background. The button becomes its result instead of showing a toast. Animate changes, not screens appearing. Joining things spring; serious things (report, leave, delete, saying no, errors) don't move beyond the press. With reduced motion, loops stop and movement becomes a fade or an instant change, but every state is still shown.

## Type

- **Display:** Dela Gothic One, the wordmark's face, for titles and plan or crew names. One weight: never set a font weight on it.
- **Body:** Zen Kaku Gothic New (400, 500, 700), which sets Japanese and Latin with the same rhythm. In the app each weight is its own family, and `ThemedText` picks it from the style's `fontWeight`. Inputs set `BodyFont.regular` themselves.
- The admin bundles both fonts with `@fontsource` (its CSP allows only `'self'`) and loads only 400 and 700. The marketing page loads them from Google Fonts.

## Switching the ground later

Ai was chosen on 2026-10-25, with three alternatives kept open: Sora 空 (sky), Yūyake 夕焼け (dusk) and Koke 苔 (moss). Their exact values are in the design system artifact under "Alternative grounds". A switch changes only ground, surface, surface-sunk, line, text-muted, accent, and the dark text and link, in the three files above. Recheck line-strong for 3:1 afterwards. Components refer to roles, never to "indigo", so nothing else changes.
