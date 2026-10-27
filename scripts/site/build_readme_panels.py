#!/usr/bin/env python3
"""Builds the animated app panels in the README from the marketing site's
app fragments, as SVGs in assets/readme/ (a light and a dark one each).

    python3 scripts/site/build_readme_panels.py

Each panel is the fragment's own markup and the site's CSS inside an SVG
<foreignObject>, with the fonts it uses inlined (subset by Google Fonts to
the panel's characters). A README can't run the site's script, and nothing
scrolls into view there, so each scene the script plays once becomes a CSS
loop: the timelines below are the site's, in the same milliseconds, as
transitions between states. The markup holds each scene's end, which is what
shows with reduced motion. The walker follows the route with SMIL instead.

GitHub serves repo files with `default-src 'none'`, but browsers don't apply
that to an SVG shown as an image: inlined fonts, foreignObject and CSS
animation all render (checked in Chrome, Firefox and WebKit). An SVG in an
<img> follows the system's colour scheme, not GitHub's theme, so each panel
comes in a light and a dark file for a <picture>, with the theme pinned.

Needs Chrome to measure each panel's height (set CHROME if it isn't at the
usual macOS path), and network access for the fonts. Rerun it after changing
a fragment on the site.
"""

from __future__ import annotations

import base64
import html
import json
import os
import re
import shutil
import subprocess
import tempfile
import urllib.parse
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SITE = ROOT / "site/index.html"
OUT = ROOT / "assets/readme"
WIDTH = 400  # the fragment's width beside the text on the site
PAD = 2      # room for the card's border to antialias
FONTS = ["Zen+Kaku+Gothic+New:wght@400;500;700", "Dela+Gothic+One"]
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"

# The site's easings (apps/mobile/src/constants/motion.ts).
SPRING = "cubic-bezier(.3, 1.6, .5, 1)"     # gather: a small overshoot
DRAW = "cubic-bezier(.4, 0, .2, 1)"
RISE = "cubic-bezier(.2, .8, .2, 1)"
ROLL = "cubic-bezier(.3, 1.3, .5, 1)"
INOUT = "cubic-bezier(.45, 0, .55, 1)"


# --- Timelines ---------------------------------------------------------------

class Loop:
    """One panel's loop: keyframes built from timed transitions, as the
    site's script changes classes and CSS transitions run between them."""

    def __init__(self, prefix: str, length: int):
        self.prefix, self.length, self.css, self.rules, self.count = prefix, length, [], {}, 0

    def track(self, selector: str, start: str, events: list[tuple]) -> None:
        """Animates `selector` from `start` through `events`, each
        (time ms, duration ms, declarations[, easing]): a transition to the
        declarations beginning at that time. A duration of 0 is a jump."""
        frames, current, last = [(0, start, None)], start, 0
        for at, duration, value, *ease in events:
            at = max(at, last)
            frames.append((at, current, (ease[0] if ease else "ease") if duration else "step-end"))
            last = at + max(duration, 1)
            frames.append((last, value, None))
            current = value
        if last > self.length:
            raise ValueError(f"{selector}: runs past the loop ({last} > {self.length} ms)")
        frames.append((self.length, current, None))
        self.count += 1
        name = f"{self.prefix}{self.count}"
        body = []
        for at, value, ease in frames:
            timing = f" animation-timing-function: {ease};" if ease else ""
            body.append(f"{at / self.length * 100:.3f}% {{ {value};{timing} }}")
        self.css.append(f"@keyframes {name} {{ {' '.join(body)} }}")
        self.rules.setdefault(selector, []).append(name)

    def steps(self, selector: str, start: str, changes: list[tuple[int, str]]) -> None:
        self.track(selector, start, [(at, 0, value) for at, value in changes])

    def stylesheet(self) -> str:
        rules = [f"{sel} {{ animation: {', '.join(f'{n} {self.length}ms linear infinite' for n in names)}; }}"
                 for sel, names in self.rules.items()]
        return "\n".join(self.css + rules)


def plan() -> tuple[Loop, callable]:
    # The route draws itself (routeDraw, 1.6 s) with each stop popping in as
    # the line reaches it, then the walker strolls it once (walk, 5.2 s).
    loop, draw, walk = Loop("pl", 10000), 500, 2300
    back = loop.length - 600
    loop.track(".map .route, .map .route-track", "stroke-dasharray: 1; stroke-dashoffset: 1",
               [(draw, 1600, "stroke-dasharray: 1; stroke-dashoffset: 0", "cubic-bezier(.4, 0, .4, 1)"),
                (back, 500, "stroke-dasharray: 1; stroke-dashoffset: 1", INOUT)])
    for n, at in enumerate([0, .93, 1], 1):
        loop.track(f".map .stop:nth-of-type({n})", "transform: scale(0)",
                   [(draw + int(at * 1600), 450, "transform: scale(1)", SPRING), (back, 300, "transform: scale(0)", INOUT)])
    for n, delay in enumerate([0, 800, 1600], 1):
        loop.track(f".stops li:nth-child({n})::before", "transform: translateX(-50%) scale(0)",
                   [(draw + delay, 450, "transform: translateX(-50%) scale(1)", SPRING),
                    (back, 300, "transform: translateX(-50%) scale(0)", INOUT)])
    for n, delay in enumerate([0, 800], 1):
        loop.track(f".stops li:nth-child({n})::after", "transform: translateX(-50%) scaleY(0); transform-origin: top",
                   [(draw + delay, 800, "transform: translateX(-50%) scaleY(1); transform-origin: top", "linear"),
                    (back, 300, "transform: translateX(-50%) scaleY(0); transform-origin: top", INOUT)])
    # Fades in leaving the station and out at the café.
    loop.track(".map .walker-dot", "opacity: 0",
               [(walk, 250, "opacity: 1"), (walk + 4950, 250, "opacity: 0")])
    a, b = walk / loop.length, (walk + 5200) / loop.length
    motion = (f'<animateMotion dur="{loop.length}ms" repeatCount="indefinite" calcMode="spline" '
              f'keyPoints="0;0;1;1" keyTimes="0;{a:.4f};{b:.4f};1" keySplines="0 0 1 1;.37 0 .63 1;0 0 1 1">'
              f'<mpath href="#route" xlink:href="#route"/></animateMotion>')

    def patch(fig: str) -> str:
        return fig.replace('<circle class="walker-dot" r="8"/>', f'<circle class="walker-dot" r="8">{motion}</circle>')
    return loop, patch


def date() -> tuple[Loop, callable]:
    # The picked slot waits as an ordinary cell, then swells once and settles
    # as it turns gold (the site's pick: .7 s, colour in .28 s).
    loop, at = Loop("dt", 5000), 900
    loop.track(".slots .pick", "background: var(--slot2); color: transparent",
               [(at, 280, "background: var(--gold); color: var(--ink)", "ease-out"),
                (loop.length - 500, 280, "background: var(--slot2); color: transparent", "ease-out")])
    loop.track(".slots .pick", "transform: scale(1)",
               [(at, 266, "transform: scale(1.07)", "cubic-bezier(.25, .1, .25, 1)"),
                (at + 266, 238, "transform: scale(.99)", INOUT), (at + 504, 196, "transform: scale(1)", INOUT)])
    return loop, lambda fig: fig


def going() -> tuple[Loop, callable]:
    # Can you go: from Aoi's and Maya's yeses (still hidden), Kenji says yes,
    # then you do and see who's coming; then Sam's yes closes the rim and a
    # glint runs round it. The times are the site script's.
    loop = Loop("go", 11000)
    reset = loop.length - 500
    seats = {  # name: (index round the table from you, flip order, said yes at)
        "you": (0, 0, 2700), "aoi": (1, 0, 0), "kenji": (2, 1, 900), "maya": (3, 2, 0), "sam": (4, 3, 5400)}
    reveal, closing = 3200, 6200
    for name, (i, k, at) in seats.items():
        seat = f'.seat[data-seat="{name}"]'
        me = name == "you"
        yes = "border-color: var(--gold); " + ("background: var(--gold); color: var(--ink)" if me else "background: var(--raised); color: var(--text)")
        undecided = "border-color: transparent; background: var(--surface); color: " + ("var(--when)" if me else "var(--text)")
        transform = []
        if at:
            loop.track(f"{seat} b", undecided, [(at, 180, yes), (reset, 180, undecided)])
            loop.track(f"{seat} b::before", "opacity: 1", [(at, 180, "opacity: 0"), (reset, 180, "opacity: 1")])
            loop.track(f'.arc[data-seat="{name}"] path', "stroke-dashoffset: 1",
                       [(at, 450, "stroke-dashoffset: 0", DRAW), (reset, 450, "stroke-dashoffset: 1", DRAW)])
            # Ties with a spring and a ring easing out (breathe, .7 s).
            transform += [(at, 0, "transform: scale(.4, .4)"), (at, 500, "transform: scale(1, 1)", SPRING)]
        ring = [(at, 0, "opacity: .7; transform: scale(1)"), (at, 490, "opacity: 0; transform: scale(1.9)", "ease-out")] if at else []
        if me:  # your seat breathes while you decide (breathe, 2.4 s)
            ring = [(300, 0, "opacity: .7; transform: scale(1)"), (300, 1680, "opacity: 0; transform: scale(1.9)", "ease-out")] + ring
        if ring:
            loop.track(f"{seat} b::after", "opacity: 0; transform: scale(1)", ring)
        if not me:
            # Turning over: edge-on as the tick becomes a name; who said yes
            # shows from your yes (or theirs, if later).
            flip = reveal + k * 90
            transform += [(flip, 200, "transform: scale(0, 1)", "ease-in-out"), (flip + 200, 200, "transform: scale(1, 1)", "ease-in-out")]
            known = flip + 200
            loop.steps(f"{seat} .who", "visibility: hidden", [(max(at, known), "visibility: visible"), (reset, "visibility: hidden")])
            if at < known:
                loop.steps(f"{seat} .hid", "visibility: hidden" if at else "visibility: visible",
                           ([(at, "visibility: visible")] if at else []) + [(known, "visibility: hidden"), (reset, "visibility: visible" if not at else "visibility: hidden")])
            else:
                loop.steps(f"{seat} .hid", "visibility: hidden", [])
        # The thread closes: each seat bumps as the glint passes.
        bump = closing + i * 260
        transform += [(bump, 200, "transform: scale(1.14, 1.14)", SPRING), (bump + 200, 300, "transform: scale(1, 1)", SPRING)]
        loop.track(f"{seat} b", "transform: scale(1, 1)", sorted(transform, key=lambda e: e[0]))
    loop.track(".digits", "transform: translateY(-2.6em)",
               [(900, 500, "transform: translateY(-3.9em)", ROLL), (2700, 500, "transform: translateY(-5.2em)", ROLL),
                (5400, 500, "transform: translateY(-6.5em)", ROLL), (reset, 500, "transform: translateY(-2.6em)", ROLL)])
    # The status line: the old one lifts away as the new one rises in.
    lines = [(-1, 900), (900, reveal), (reveal, 5400), (5400, reset)]
    hidden, shown, gone = "opacity: 0; transform: translateY(.5rem)", "opacity: 1; transform: none", "opacity: 0; transform: translateY(-.4rem)"
    for n, (start, end) in enumerate(lines, 1):
        events = [] if start < 0 else [(start, 280, shown, RISE)]
        events += [(end, 180, gone, "ease"), (end + 180, 0, hidden)]
        if start < 0:
            events.append((reset + 200, 280, shown, RISE))
        loop.track(f".status .s{n}", shown if start < 0 else hidden, events)
    # Confirming: the press, a gold ring easing out, the button becomes its
    # result with its tick drawing, and what happens next rises in.
    loop.track(".invite .result", "transform: scale(1)",
               [(2600, 100, "transform: scale(.97)"), (2700, 100, "transform: scale(1)")])
    loop.track(".invite .result::after", "opacity: 0; box-shadow: 0 0 0 0 var(--gold)",
               [(2600, 0, "opacity: .7; box-shadow: 0 0 0 0 var(--gold)"), (2600, 600, "opacity: 0; box-shadow: 0 0 0 .7rem var(--gold)", "ease-out")])
    for part in [".result", ".then"]:
        loop.steps(f".invite {part} .ask", "visibility: visible", [(2700, "visibility: hidden"), (reset, "visibility: visible")])
    loop.track(".invite .result .done", "visibility: hidden; opacity: 0",
               [(2700, 0, "visibility: visible; opacity: 0"), (2700, 180, "visibility: visible; opacity: 1"), (reset, 0, "visibility: hidden; opacity: 0")])
    loop.track(".invite .then .done", "visibility: hidden; opacity: 0; transform: translateY(.5rem)",
               [(2950, 0, "visibility: visible; opacity: 0; transform: translateY(.5rem)"), (2950, 280, "visibility: visible; opacity: 1; transform: none", RISE),
                (reset, 0, "visibility: hidden; opacity: 0; transform: translateY(.5rem)")])
    loop.track(".check path", "stroke-dasharray: 1; stroke-dashoffset: 1",
               [(2800, 450, "stroke-dasharray: 1; stroke-dashoffset: 0", "ease-out"), (reset, 0, "stroke-dasharray: 1; stroke-dashoffset: 1")])
    loop.track(".glint", "stroke-dashoffset: 7", [(closing, 1300, "stroke-dashoffset: -93", INOUT)])
    loop.track(".glint", "opacity: 0", [(closing, 156, "opacity: 1"), (closing + 1105, 195, "opacity: 0")])

    def patch(fig: str) -> str:
        texts = ["Two of your group have said yes.", "Someone else just said yes.",
                 "Who is going: Aoi, Kenji, Maya and you.", "Enough people confirmed, so it is going ahead."]
        spans = "".join(f'<span class="s{n}">{t}</span>' for n, t in enumerate(texts, 1))
        return re.sub(r'<p class="status">.*?</p>', f'<p class="status">{spans}</p>', fig)
    return loop, patch


def day(fig_text: str) -> tuple[Loop, callable]:
    # The day: Maya's message is there; you type a follow-up and send it
    # (press 100 ms, the bubble rises in 280 ms), then Kenji starts typing.
    loop = Loop("dy", 9500)
    text = html.unescape(re.search(r'<p class="msg mine">(.*?)</p>', fig_text).group(1))
    keys, at = [], 900
    for n, ch in enumerate(text):
        keys.append(at)
        at += 70 if ch == " " else 40
    press = keys[-1] + 450
    sent = press + 100
    kenji = sent + 1400
    clear = loop.length - 1000
    for n, key in enumerate(keys, 1):
        loop.steps(f".typed i:nth-child({n})", "font-size: 0", [(key, "font-size: .9rem"), (sent, "font-size: 0")])
    loop.steps(".typed::after", "font-size: 0", [(900, "font-size: .9rem"), (sent, "font-size: 0")])
    loop.steps(".composer .placeholder", "font-size: .9rem", [(900, "font-size: 0"), (sent, "font-size: .9rem")])
    loop.track(".composer .send", "opacity: .4", [(900, 180, "opacity: 1"), (sent, 180, "opacity: .4")])
    loop.track(".composer .send", "transform: scale(1)", [(press, 100, "transform: scale(.97)"), (sent, 100, "transform: scale(1)")])
    # New messages push the old ones up: a hidden message takes no room (the
    # negative margin cancels the column's gap) and opens as it rises in.
    folded = "max-height: 0; padding-block: 0; margin-top: -.5rem; opacity: 0; transform: translateY(.5rem)"
    loop.track(".msg.mine", folded, [(sent, 280, "max-height: 6rem; padding-block: .5rem; margin-top: 0; opacity: 1; transform: none", RISE),
                                     (clear, 400, folded, INOUT)])
    folded = "max-height: 0; margin-top: -.5rem; opacity: 0"
    loop.track(".chat .typing", folded, [(kenji, 180, "max-height: 2rem; margin-top: 0; opacity: 1"), (clear, 400, folded, INOUT)])

    def patch(fig: str) -> str:
        keys = "".join(f"<i>{html.escape(ch)}</i>" for ch in text)
        return fig.replace('<span class="typed"></span>', f'<span class="typed">{keys}</span>')
    return loop, patch


def again() -> tuple[Loop, callable]:
    # How was it: the picks tick themselves, one after another.
    loop = Loop("ag", 5000)
    for n, at in [(1, 700), (3, 1100), (4, 1500)]:
        tick = f".picks li:nth-child({n}) .tick"
        loop.track(tick, "background: transparent; border-color: var(--line-strong)",
                   [(at, 180, "background: var(--gold); border-color: var(--gold)"),
                    (loop.length - 600, 180, "background: transparent; border-color: var(--line-strong)")])
        loop.track(f"{tick}::after", "transform: translate(-50%, -50%) rotate(45deg) scale(0)",
                   [(at, 450, "transform: translate(-50%, -50%) rotate(45deg) scale(1)", SPRING),
                    (loop.length - 600, 180, "transform: translate(-50%, -50%) rotate(45deg) scale(0)")])
    return loop, lambda fig: fig


def crew() -> tuple[Loop, callable]:
    # The crew: from one yes, Maya says yes, then you do; the third yes fills
    # the thread (draw, 450 ms), it starts and everyone hops.
    loop = Loop("cr", 7500)
    reset = loop.length - 700
    maya, me, started = 800, 2100, 2600
    loop.track(".members .track.fill", "transform: scaleX(.3333)",
               [(maya, 450, "transform: scaleX(.6667)", DRAW), (me, 450, "transform: scaleX(1)", DRAW), (reset, 450, "transform: scaleX(.3333)", DRAW)])
    loop.steps(".counter .n i:nth-child(1)", "visibility: visible", [(maya, "visibility: hidden"), (reset, "visibility: visible")])
    loop.steps(".counter .n i:nth-child(2)", "visibility: hidden", [(maya, "visibility: visible"), (me, "visibility: hidden")])
    loop.steps(".counter .n i:nth-child(3)", "visibility: hidden", [(me, "visibility: visible"), (reset, "visibility: hidden")])
    for n, at in [(2, maya), (3, me)]:
        loop.track(f".bars i:nth-child({n})", "background: var(--raised)", [(at, 180, "background: var(--gold)"), (reset, 180, "background: var(--raised)")])
    # Members: Aoi, Maya, Sam, you (after the two tracks).
    for child, at, mine in [(4, maya, False), (6, me, True)]:
        member = f".member:nth-child({child})"
        yes = "border-color: var(--gold)" + ("; background: var(--gold); color: var(--ink)" if mine else "")
        no = "border-color: transparent" + ("; background: var(--surface); color: var(--text)" if mine else "")
        loop.track(f"{member} b", no, [(at, 180, yes), (reset, 180, no)])
        loop.track(f"{member} b", "transform: scale(1)", [(at, 0, "transform: scale(.4)"), (at, 500, "transform: scale(1)", SPRING)])
        loop.track(f"{member} b::before", "opacity: 1", [(at, 180, "opacity: 0"), (reset, 180, "opacity: 1")])
        loop.steps(f"{member} .no", "visibility: visible", [(at, "visibility: hidden"), (reset, "visibility: visible")])
        loop.steps(f"{member} .ok", "visibility: hidden", [(at, "visibility: visible"), (reset, "visibility: hidden")])
    loop.track(".crew-card .fake-btn.primary", "transform: scale(1)", [(me - 100, 100, "transform: scale(.97)"), (me, 100, "transform: scale(1)")])
    loop.steps(".crew-card .swap .ask", "visibility: visible", [(started, "visibility: hidden"), (reset, "visibility: visible")])
    loop.steps(".crew-card .swap .done", "visibility: hidden", [(started, "visibility: visible"), (reset, "visibility: hidden")])
    for i in range(4):
        hop = started + i * 70
        loop.track(f".member:nth-child({i + 3}) .av", "transform: none",
                   [(hop, 240, "transform: translateY(-10px)", "cubic-bezier(.3, 1.4, .5, 1)"), (hop + 240, 360, "transform: none", "cubic-bezier(.3, 1.4, .5, 1)")])

    def patch(fig: str) -> str:
        return fig.replace('<span class="n">3</span>', '<span class="n"><i>1</i><i>2</i><i>3</i></span>')
    return loop, patch


# Per panel: file name, the fragment's aria-label, its loop, and CSS for the
# markup the loop adds (whose resting state is the scene's end).
PANELS = [
    ("plan", "A plan in the app", plan, ""),
    ("date", "When people who signed up can go", date, ""),
    ("going", "Confirming a place in a group of five", going,
     ".status .s1, .status .s2, .status .s3 { opacity: 0; }"),
    ("day", "The outing on the day, with the group chat", None,
     # The typed text: each key is its own span, sized 0 until typed, so the
     # one-line box keeps showing the line being typed.
     # (.fo outranks the site's .chat:not(.s-typing) .typed, which hides it.)
     ".fo .chat .typed { display: block; font-size: 0; line-height: 1.45; } .typed i { font-style: normal; font-size: 0; }"
     " .chat .typed::after { content: \"\"; border-left: 1.5px solid currentColor; margin-left: 1px; font-size: 0; }"
     " .chat .msg.mine, .chat .typing { overflow: hidden; }"),
    ("again", "Choosing who to meet again", again, ""),
    ("crew", "A crew offer that has just started", crew,
     ".counter .n { display: inline-grid; } .counter .n i { grid-area: 1 / 1; font-style: normal; }"
     " .counter .n i:not(:last-child) { visibility: hidden; }"),
]


# --- Building -----------------------------------------------------------------

def fragment(page: str, label: str) -> str:
    start = page.index(f'aria-label="{label}"')
    start = page.rindex("<figure", 0, start)
    return page[start:page.index("</figure>", start) + len("</figure>")]


def xhtml(fig: str) -> str:
    """The fragment as XHTML: no comments, no named entities, inline SVGs in
    the SVG namespace."""
    fig = re.sub(r"<!--.*?-->\s*", "", fig, flags=re.S)
    parts = re.split(r"(<[^>]+>)", fig)
    for n, part in enumerate(parts):
        if part.startswith("<"):
            parts[n] = html.unescape(part).replace("&", "&amp;")
        else:
            parts[n] = html.escape(html.unescape(part), quote=False)
    fig = "".join(parts)
    return fig.replace("<svg ", '<svg xmlns="http://www.w3.org/2000/svg" ')


def minify(css: str) -> str:
    css = re.sub(r"/\*.*?\*/", "", css, flags=re.S)
    return re.sub(r"\s+", " ", css).strip()


def desktop(css: str) -> str:
    """Drops the site's phone-only rules (@media (max-width: …)). Media
    queries in an SVG image see the image's width, not the screen's, so they
    would put these desktop-width fragments in their phone layout."""
    out, i = [], 0
    for m in re.finditer(r"@media[^{]*max-width[^{]*\{", css):
        if m.start() < i:
            continue
        depth, j = 1, m.end()
        while depth:
            depth += {"{": 1, "}": -1}.get(css[j], 0)
            j += 1
        out.append(css[i:m.start()])
        i = j
    return "".join(out) + css[i:]


def fonts(chars: str) -> str:
    out = []
    for family in FONTS:
        url = f"https://fonts.googleapis.com/css2?family={family}&text={urllib.parse.quote(chars)}"
        css = urllib.request.urlopen(urllib.request.Request(url, headers={"User-Agent": UA})).read().decode()
        for src in re.findall(r"url\((https://[^)]+)\)", css):
            data = urllib.request.urlopen(urllib.request.Request(src, headers={"User-Agent": UA})).read()
            css = css.replace(src, "data:font/woff2;base64," + base64.b64encode(data).decode())
        out.append(css)
    return minify("".join(out))


def chrome() -> str:
    for candidate in [os.environ.get("CHROME"), "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
                      shutil.which("google-chrome"), shutil.which("chromium")]:
        if candidate and Path(candidate).exists():
            return candidate
    raise SystemExit("Chrome not found: set CHROME to its path")


def measure(bodies: list[str]) -> list[int]:
    """Each panel's height at the site's width, in headless Chrome."""
    page = ("<!doctype html><meta charset=utf-8><style>* { animation: none !important; }</style>"
            # Parsed as HTML here, where a CDATA marker would break the first rule.
            + "".join(f'<div style="width: {WIDTH}px">{b.replace("<![CDATA[", "").replace("]]>", "")}</div>' for b in bodies)
            + '<pre id="h"></pre><script>document.fonts.ready.then(() => { document.getElementById("h").textContent ='
              ' JSON.stringify([...document.querySelectorAll(".fo > figure")].map(f => Math.ceil(f.getBoundingClientRect().height))); });</script>')
    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / "measure.html"
        path.write_text(page)
        dom = subprocess.run([chrome(), "--headless=new", "--disable-gpu", f"--user-data-dir={tmp}/profile", "--dump-dom",
                              "--virtual-time-budget=10000", f"--window-size={WIDTH + 2 * PAD},2000", path.as_uri()], capture_output=True, text=True, check=True).stdout
    return json.loads(re.search(r'<pre id="h">(.*?)</pre>', dom).group(1))


def main() -> None:
    page = SITE.read_text()
    css = desktop(minify(re.search(r"<style>(.*?)</style>", page, re.S).group(1)))
    bodies, built = [], []
    for name, label, make, extra in PANELS:
        fig = fragment(page, label)
        loop, patch = day(fig) if name == "day" else make()
        fig = xhtml(patch(fig))
        chars = "".join(sorted(set(html.unescape(re.sub(r"<[^>]+>", "", fig))))) + "0123456789"
        style = "\n".join([fonts(chars), css,
                           # The fragment's page around it, and the scene's end without motion.
                           ".fo { font-family: var(--sans); color: var(--text); line-height: 1.6; -webkit-font-smoothing: antialiased; }",
                           extra, loop.stylesheet(),
                           "@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation: none !important; } }"])
        bodies.append(f'<div xmlns="http://www.w3.org/1999/xhtml" class="fo"><style><![CDATA[{style}]]></style>{fig}</div>')
        built.append((name, label))
    heights = measure(bodies)
    OUT.mkdir(parents=True, exist_ok=True)
    for (name, label), body, height in zip(built, bodies, heights):
        w, h = WIDTH + 2 * PAD, height + 2 * PAD
        for theme in ["light", "dark"]:
            svg = (f'<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" data-theme="{theme}"'
                   f' width="{w}" height="{h}" viewBox="0 0 {w} {h}" role="img" aria-label="{html.escape(label)}">'
                   f'<foreignObject x="{PAD}" y="{PAD}" width="{WIDTH}" height="{height}">{body}</foreignObject></svg>\n')
            path = OUT / f"{name}-{theme}.svg"
            path.write_text(svg)
            print(f"{path.relative_to(ROOT)}: {w}x{h}, {len(svg.encode()) / 1024:.0f} KB")


if __name__ == "__main__":
    main()
