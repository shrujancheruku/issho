# Screen design canvas: the render pipeline

Throwaway, from the ticket "The design canvas and the v1 board inventory" ([#645](https://github.com/shrujancheruku/issho/issues/645)) on the map "Wayfinder: draw the screens for the thin loop and plan-making" ([#644](https://github.com/shrujancheruku/issho/issues/644)). The branch is never merged.

The screens are drawn on one Design canvas, **Issho v2 screens** (<https://claude.ai/artifact/CN3EEuP5VM5ntgqkWMQXRY>). The canvas is the source of truth. This folder holds what renders its boards outside claude.ai, so every board can be screenshotted in light, dark and Vietnamese before the owner sees it.

| File | What it is |
|---|---|
| `render.py` | Renders every board of a canvas folder to PNGs in light and dark and reports layout problems. |
| `support.js` | A local stand-in for the canvas runtime: `{{holes}}`, `<sc-if>`, `<sc-for>`, `<dc-import>`, events, `setState`, the `<helmet>`. It also measures each board. |
| `plans.json` | The twelve plans from the plan bar's prototype, as sample content. Areas and activity types are guesses until plan-making sets them. |

## Rendering the canvas

1. Pull the canvas's files with the Artifact tool's `read` action and `paths`: `project/canvas.json`, the `project/ds/issho/` files (`tokens.css`, `tokens.json`, `components/bundle.css`) and the boards you changed. The files land in `scratchpad/artifact-files/<id>/project/…`.
2. Run the script against that folder. It needs Google Chrome; set `CHROME` if it isn't at the macOS default path.

   ```sh
   python3 render.py <folder> --out <shots>                       # every board, light and dark
   python3 render.py <folder> --out <shots> --only Plans Plans-vi # just these
   ```

3. Read every PNG. The checks catch geometry, not sense.

Each board's theme comes from the URL (`?theme=dark`). A board named `<Name>-dark` only shows another board dark on the canvas, so it is skipped: rendering `<Name>` in dark covers it. A Vietnamese board is its own file, `<Name>-vi`, and is rendered in both themes.

The script exits 1 when a board has problems. It reports:

- an element off the side of the board;
- a button, link or input inside the 16 px gutter, on phone boards;
- text or a control under a fixed top or bottom bar;
- text below the bottom of the board, and content taller than the board;
- a script error, or a board the stand-in never finished rendering.

## What the stand-in doesn't do

It is not the canvas's runtime. It doesn't mount a design system's React components (Issho's design system has none, only `tokens.css` and `bundle.css`), and it renders `<dc-import>` as `display: contents`. A board that renders here can still differ on the canvas, so look at the canvas too before sign-off.
