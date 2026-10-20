"""Generates the Issho logo SVGs in assets/brand/ from Noto Serif JP outlines.

Concept: 一緒 ("together"). 一 ("one") is drawn as a single gold thread. In 緒,
the thread radical 糸 is gold, and the 日 ("sun") at the bottom of 者 ("person")
is replaced by the red hinomaru disc. The mark reads: one thread joining people
under one sun.

Usage (from repo root):
    python3 -m venv /tmp/brand-venv && /tmp/brand-venv/bin/pip install fonttools
    curl -L -o /tmp/NotoSerifJP.ttf "https://github.com/google/fonts/raw/main/ofl/notoserifjp/NotoSerifJP%5Bwght%5D.ttf"
    /tmp/brand-venv/bin/python scripts/brand/build_logo.py /tmp/NotoSerifJP.ttf assets/brand

The font (SIL Open Font License 1.1) is not committed; only the generated outlines are.
"""
import sys

from fontTools.pens.recordingPen import RecordingPen
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

RED, GOLD = "#BC002D", "#C9A227"  # hinomaru red and gold
INK = {"light": "#111111", "dark": "#F5EFE0"}  # dark variant uses a warm off-white ("washi") ink

# Contour indices of 緒 in Noto Serif JP (weight 800), worked out by rendering each
# contour in its own colour. If the font version changes, check these indices again.
THREAD_IDX = range(7, 15)  # 糸 radical
PERSON_IDX = range(0, 4)   # 土 + ノ of 者
SUN_IDX = (4, 5, 6)        # 日 of 者 → drawn as the hinomaru disc


def load(path, wght):
    return instantiateVariableFont(TTFont(path), {"wght": wght})


def contours(font, ch):
    """Splits a glyph into its individual closed contours, so parts can be coloured separately."""
    rec = RecordingPen()
    font.getGlyphSet()[font.getBestCmap()[ord(ch)]].draw(rec)
    out, cur = [], []
    for op, args in rec.value:
        cur.append((op, args))
        if op in ("closePath", "endPath"):
            out.append(cur)
            cur = []
    return out


def bounds(cs):
    pts = [p for c in cs for _, args in c for p in args]
    xs, ys = [p[0] for p in pts], [p[1] for p in pts]
    return min(xs), min(ys), max(xs), max(ys)


def to_path(glyphset, cs):
    pen = SVGPathPen(glyphset)
    for c in cs:
        for op, args in c:
            getattr(pen, op)(*args)
    return pen.getCommands()


def build(font_path, out_dir):
    heavy = load(font_path, 800)  # kanji
    medium = load(font_path, 600)  # wordmark
    gs = heavy.getGlyphSet()

    ichi = contours(heavy, "一")
    cho = contours(heavy, "緒")
    thread = [cho[i] for i in THREAD_IDX]
    person = [cho[i] for i in PERSON_IDX]

    # The disc sits where 日 was. It's sized from 日's box, slightly smaller than
    # the box's long side so it reads as a sun rather than a blob.
    sx0, sy0, sx1, sy1 = bounds([cho[i] for i in SUN_IDX])
    sun_cx, sun_cy = (sx0 + sx1) / 2, (sy0 + sy1) / 2 + 10
    sun_r = 0.43 * max(sx1 - sx0, sy1 - sy0)

    def cho_group(ink):
        # Font units are y-up; scale(1,-1) flips them into SVG's y-down space.
        # The sun is drawn first so the ノ stroke of 者 passes over it.
        return (f'<g transform="scale(1,-1)">'
                f'<circle cx="{sun_cx:.1f}" cy="{sun_cy:.1f}" r="{sun_r:.1f}" fill="{RED}"/>'
                f'<path d="{to_path(gs, thread)}" fill="{GOLD}"/>'
                f'<path d="{to_path(gs, person)}" fill="{ink}"/></g>')

    def wordmark(ink):
        text, track, size = "ISSHO", 300, 0.30  # tracking in font units; size relative to the kanji
        mgs, cmap = medium.getGlyphSet(), medium.getBestCmap()
        glyphs = [mgs[cmap[ord(ch)]] for ch in text]
        total = sum(g.width for g in glyphs) + track * (len(glyphs) - 1)
        x, parts = 0, []
        for g in glyphs:
            pen = SVGPathPen(mgs)
            g.draw(pen)
            parts.append(f'<path transform="translate({x},0)" d="{pen.getCommands()}"/>')
            x += g.width + track
        ox = 1000 - total * size / 2  # centred under the 2000-unit-wide 一緒
        return (f'<g fill="{ink}" transform="translate({ox:.1f},400) scale({size},{-size})">'
                f'{"".join(parts)}</g>')

    def mark(ink):
        return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="-20 -900 1040 1040" role="img" aria-label="Issho">'
                f'<title>Issho</title>{cho_group(ink)}</svg>\n')

    def logo(ink):
        return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="-40 -920 2080 1380" role="img" aria-label="Issho 一緒">'
                f'<title>Issho 一緒</title>'
                f'<g transform="scale(1,-1)"><path d="{to_path(gs, ichi)}" fill="{GOLD}"/></g>'
                f'<g transform="translate(1000,0)">{cho_group(ink)}</g>'
                f'{wordmark(ink)}</svg>\n')

    for theme, ink in INK.items():
        sfx = "" if theme == "light" else "-dark"
        with open(f"{out_dir}/logo{sfx}.svg", "w") as fh:
            fh.write(logo(ink))
        with open(f"{out_dir}/mark{sfx}.svg", "w") as fh:
            fh.write(mark(ink))


if __name__ == "__main__":
    build(sys.argv[1], sys.argv[2])
