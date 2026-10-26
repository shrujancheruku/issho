#!/usr/bin/env python3
"""Builds the app's icons and splash image from assets/brand/mark.svg.

    python3 scripts/brand/build_app_icons.py

Needs rsvg-convert (`brew install librsvg`); otherwise only the standard
library. Rerun it whenever the mark changes (e.g. after build_logo.py), and
commit the PNGs it writes to apps/mobile/assets/images/:

    icon.png                     1024, the mark on Ai night: iOS and the default
    android-icon-foreground.png  1024, transparent, inside the adaptive icon's
                                 safe zone (app.json sets the background colour)
    android-icon-monochrome.png  1024, white silhouette: Android 13 themed icons
                                 and the notification icon
    splash-icon.png              1024, transparent (app.json sets the size and
                                 the light and dark background colours)
    favicon.png                  48, as icon.png

The mark is used as is (assets/brand/README.md): its outlines let it sit on
any background, so only the background and the margin change here.
"""

from __future__ import annotations

import re
import subprocess
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MARK = ROOT / "assets/brand/mark.svg"
OUT = ROOT / "apps/mobile/assets/images"

# Ai night, the app's dark background (apps/mobile/src/constants/theme.ts).
AI_NIGHT = "#0D1330"


def framed(svg: str, share: float, background: str | None = None, silhouette: bool = False) -> str:
    """The mark taking `share` of a square canvas's width, centred."""
    m = re.search(r'viewBox="([-\d.]+) ([-\d.]+) ([\d.]+) ([\d.]+)"', svg)
    x, y, w, h = (float(v) for v in m.groups())
    side = max(w, h) / share
    nx, ny = x - (side - w) / 2, y - (side - h) / 2
    svg = svg.replace(m.group(0), f'viewBox="{nx} {ny} {side} {side}"', 1)
    extra = ""
    if background:
        extra += f'<rect x="{nx}" y="{ny}" width="{side}" height="{side}" fill="{background}"/>'
    if silhouette:
        # CSS beats the mark's fill and stroke attributes: one white shape.
        extra += "<style>path,circle{fill:#fff;stroke:#fff}</style>"
    return re.sub(r"(<svg[^>]*>)", lambda t: t.group(1) + extra, svg, count=1)


def render(svg: str, size: int, name: str) -> None:
    with tempfile.NamedTemporaryFile("w", suffix=".svg", encoding="utf-8") as f:
        f.write(svg)
        f.flush()
        subprocess.run(["rsvg-convert", "-w", str(size), "-h", str(size), "-o", str(OUT / name), f.name], check=True)
    print(f"wrote {name} ({size}px)")


def main() -> None:
    mark = MARK.read_text(encoding="utf-8")
    render(framed(mark, 0.66, AI_NIGHT), 1024, "icon.png")
    # Android masks the adaptive icon to as little as the middle 66%
    # circle; a square mark fits inside it at under 47% of the canvas.
    render(framed(mark, 0.46), 1024, "android-icon-foreground.png")
    render(framed(mark, 0.46, silhouette=True), 1024, "android-icon-monochrome.png")
    render(framed(mark, 1.0), 1024, "splash-icon.png")
    render(framed(mark, 0.8, AI_NIGHT), 48, "favicon.png")


if __name__ == "__main__":
    main()
