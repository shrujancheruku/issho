#!/usr/bin/env python3
"""Builds the logo's round versions and PNG exports from assets/brand/.

    python3 scripts/brand/build_variants.py

Needs rsvg-convert (`brew install librsvg`) for the PNGs and ffmpeg for the
JPGs; otherwise only the standard library. Rerun it whenever logo.svg or mark.svg change (e.g. after
build_logo.py); `make app-icons` runs it first. It writes:

    assets/brand/logo-round-blue.svg   the lockup in an Ai night disc
    assets/brand/logo-round-white.svg  the lockup in a white disc
    assets/brand/mark-round-blue.svg   the mark in an Ai night disc
    assets/brand/mark-round-white.svg  the mark in a white disc
    assets/brand/png/*.png             each of the six, 1024 px wide
    assets/brand/jpg/*.jpg             each of the six, 1024 px wide, for places
                                       that only take JPG; JPG has no
                                       transparency, so the plain and blue round
                                       versions sit on white and the white round
                                       ones on Ai night, keeping every disc visible
    site/favicon.svg, site/logo.svg    the site's copies
    site/apple-touch-icon.png          180, the mark on an Ai night square
                                       (iOS rounds the corners itself)
    site/og-image.png                  1200x630, the lockup on Ai night, for
                                       link previews

The logo itself is never changed: its outlines already make it readable on
any background, so these only add a disc behind it.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BRAND = ROOT / "assets/brand"

# Palette (docs/design-system.md): Ai night is the app's dark canvas and the
# app icon's background; white is the light theme's surface.
DISCS = {"blue": "#0D1330", "white": "#FFFFFF"}

# Where each disc sits, in the SVG's own units: centred on the drawing's
# bounding box (so it looks balanced, rather than the mathematically smallest
# circle, which pulls the lockup off centre), with a radius 12% larger than
# the farthest painted point from that centre. Measured by rendering the
# SVGs; measure again if the shapes change.
FRAMES = {
    "mark": {"cx": 504, "cy": -381, "r": 710},
    "logo": {"cx": 1008, "cy": -226, "r": 1225},
}


def round_version(svg: str, cx: float, cy: float, r: float, fill: str) -> str:
    """The SVG on a disc of `fill`, with the viewBox cropped to the disc."""
    svg = re.sub(r'viewBox="[^"]*"', f'viewBox="{cx - r:g} {cy - r:g} {2 * r:g} {2 * r:g}"', svg, count=1)
    disc = f'<circle cx="{cx:g}" cy="{cy:g}" r="{r:g}" fill="{fill}"/>'
    # After <title>, so the disc is painted first, under the logo.
    return re.sub(r"(</title>)", lambda m: m.group(1) + disc, svg, count=1)


def on_canvas(svg: str, width: float, height: float, share: float, fill: str) -> str:
    """The SVG centred on a width x height canvas of `fill`, taking `share` of its height."""
    m = re.search(r'viewBox="([-\d.]+) ([-\d.]+) ([\d.]+) ([\d.]+)"', svg)
    x, y, w, h = (float(v) for v in m.groups())
    scale = h / share / height  # SVG units per canvas pixel
    cw, ch = width * scale, height * scale
    nx, ny = x - (cw - w) / 2, y - (ch - h) / 2
    svg = svg.replace(m.group(0), f'viewBox="{nx:g} {ny:g} {cw:g} {ch:g}"', 1)
    rect = f'<rect x="{nx:g}" y="{ny:g}" width="{cw:g}" height="{ch:g}" fill="{fill}"/>'
    return re.sub(r"(</title>)", lambda t: t.group(1) + rect, svg, count=1)


def render(svg: str, out: Path, width: int, height: int) -> None:
    subprocess.run(["rsvg-convert", "-w", str(width), "-h", str(height), "-o", str(out)], input=svg.encode(), check=True)


def png(svg_path: Path, out: Path, width: int = 1024) -> None:
    out.parent.mkdir(exist_ok=True)
    subprocess.run(["rsvg-convert", "-w", str(width), "-o", str(out), str(svg_path)], check=True)


def jpg(svg_path: Path, out: Path, background: str, width: int = 1024) -> None:
    out.parent.mkdir(exist_ok=True)
    svg = svg_path.read_text(encoding="utf-8")
    if "-round-" not in svg_path.stem:
        # The plain files are cropped almost to the drawing; on a solid
        # background that looks cramped, so give them an 8% margin all round.
        w, h = (float(v) for v in re.search(r'viewBox="[-\d.]+ [-\d.]+ ([\d.]+) ([\d.]+)"', svg).groups())
        svg = on_canvas(svg, width, width * h / w, 0.84, background)
    flat = subprocess.run(["rsvg-convert", "-w", str(width), "-b", background], input=svg.encode(), capture_output=True, check=True).stdout
    subprocess.run(["ffmpeg", "-loglevel", "error", "-y", "-f", "png_pipe", "-i", "-", "-q:v", "2", str(out)], input=flat, check=True)


def main() -> None:
    written = []
    for name, frame in FRAMES.items():
        svg = (BRAND / f"{name}.svg").read_text(encoding="utf-8")
        written.append(BRAND / f"{name}.svg")
        for colour, fill in DISCS.items():
            path = BRAND / f"{name}-round-{colour}.svg"
            path.write_text(round_version(svg, **frame, fill=fill), encoding="utf-8")
            written.append(path)
    for path in written:
        png(path, BRAND / "png" / f"{path.stem}.png")
        background = DISCS["blue"] if path.stem.endswith("-white") else DISCS["white"]
        jpg(path, BRAND / "jpg" / f"{path.stem}.jpg", background)
        print(f"wrote {path.relative_to(ROOT)}, png/{path.stem}.png and jpg/{path.stem}.jpg")
    # The site: the lockup as is, and the blue round mark as its favicon.
    (ROOT / "site/logo.svg").write_text((BRAND / "logo.svg").read_text(encoding="utf-8"), encoding="utf-8")
    (ROOT / "site/favicon.svg").write_text((BRAND / "mark-round-blue.svg").read_text(encoding="utf-8"), encoding="utf-8")
    print("wrote site/logo.svg and site/favicon.svg")
    mark, logo = ((BRAND / f"{n}.svg").read_text(encoding="utf-8") for n in ("mark", "logo"))
    render(on_canvas(mark, 180, 180, 0.66, DISCS["blue"]), ROOT / "site/apple-touch-icon.png", 180, 180)
    render(on_canvas(logo, 1200, 630, 0.62, DISCS["blue"]), ROOT / "site/og-image.png", 1200, 630)
    print("wrote site/apple-touch-icon.png and site/og-image.png")


if __name__ == "__main__":
    main()
