#!/usr/bin/env python3
"""Draws the Kichijoji map in the marketing site's plan fragment from
OpenStreetMap data, and writes it into site/index.html between the
<!-- map:start --> and <!-- map:end --> markers.

    python3 scripts/site/build_map.py              # fetches the area from OSM
    python3 scripts/site/build_map.py area.osm     # or uses a saved copy

Only the standard library. The walk is routed along real paths, so the map
and the fragment's stops agree with the place:

    1. Kichijoji Station, south (park) exit
    2. Inokashira Park: down Suehiro-dori and Benten-dori to the east end of
       the pond, then once around it (the south shore west, round the
       western arm, back along the north shore) to the north end of Nanai
       Bridge
    3. A café on Nanaibashi-dori, back up towards the station

The map is inline SVG so it takes the page's light and dark colours (the
--map-* variables) and the route can draw itself with a walker on it.
Map data © OpenStreetMap contributors (ODbL).
"""

from __future__ import annotations

import heapq
import math
import re
import sys
import urllib.request
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SITE = ROOT / "site/index.html"

# The area drawn (and fetched): the route with the whole pond around it.
SOUTH, NORTH, WEST, EAST = 35.6976, 35.7037, 139.5742, 139.5834
FETCH = "https://api.openstreetmap.org/api/0.6/map?bbox={},{},{},{}".format(WEST - .0004, SOUTH - .0004, EAST + .0004, NORTH + .0004)

# Where the walk starts, turns and stops. Each snaps to the nearest walkable
# node; the park stop is the north end of Nanai Bridge (way 31877607), so the
# walk crosses nothing twice.
START = (35.7027305, 139.5797356)  # 南口 / park exit (node 9444122974)
# Around the pond, in order: its east end by Benzaiten, two points on the
# south shore, the tip of the western arm, and the north shore.
AROUND_POND = [(35.6988, 139.5806), (35.6984, 139.5785), (35.6990, 139.5760), (35.6996, 139.5740), (35.7013, 139.5755)]
BRIDGE_WAY = "31877607"            # 七井橋 (Nanai Bridge)
CAFE = (35.70117, 139.57853)       # partway up 七井橋通り (Nanaibashi-dori)

WALK = {"footway", "path", "steps", "pedestrian", "service", "living_street", "residential",
        "unclassified", "tertiary", "secondary", "primary", "cycleway", "track"}
MAJOR = {"primary", "secondary", "tertiary"}
MINOR = {"residential", "unclassified", "pedestrian", "living_street", "service"}
PATHS = {"footway", "path", "steps", "cycleway", "track"}

COS = math.cos(math.radians((SOUTH + NORTH) / 2))
M = 111_320  # metres per degree of latitude


def xy(p):
    """Metres from the map's top-left corner."""
    return ((p[1] - WEST) * M * COS, (NORTH - p[0]) * M)


def simplify(pts, tol=1.5):
    """Douglas-Peucker in metres: keeps the shape, drops the bytes."""
    if len(pts) < 3:
        return pts
    if pts[0] == pts[-1]:
        # A closed ring: its ends coincide, so split it at the point farthest
        # from the start and simplify the halves.
        k = max(range(len(pts)), key=lambda i: math.hypot(pts[i][0] - pts[0][0], pts[i][1] - pts[0][1]))
        return simplify(pts[: k + 1], tol)[:-1] + simplify(pts[k:], tol)
    (x1, y1), (x2, y2) = pts[0], pts[-1]
    dx, dy = x2 - x1, y2 - y1
    norm = math.hypot(dx, dy) or 1e-9
    i, far = 0, -1.0
    for k, (x, y) in enumerate(pts[1:-1], 1):
        d = abs(dy * x - dx * y + x2 * y1 - y2 * x1) / norm
        if d > far:
            i, far = k, d
    if far <= tol:
        return [pts[0], pts[-1]]
    return simplify(pts[: i + 1], tol)[:-1] + simplify(pts[i:], tol)


def d_attr(lines, closed=False):
    out = []
    for line in lines:
        pts = simplify([xy(p) for p in line])
        if len(pts) < 2:
            continue
        out.append("M" + " ".join(f"{x:.0f} {y:.0f}" for x, y in pts) + ("Z" if closed else ""))
    return "".join(out)


def in_view(line):
    return any(SOUTH <= la <= NORTH and WEST <= lo <= EAST for la, lo in line)


def main() -> None:
    if len(sys.argv) > 1:
        raw = Path(sys.argv[1]).read_bytes()
    else:
        req = urllib.request.Request(FETCH, headers={"User-Agent": "issho-site-map/1.0"})
        raw = urllib.request.urlopen(req, timeout=120).read()
    root = ET.fromstring(raw)
    nodes = {n.get("id"): (float(n.get("lat")), float(n.get("lon"))) for n in root.iter("node")}
    tags = lambda e: {t.get("k"): t.get("v") for t in e.findall("tag")}
    ways = {w.get("id"): (tags(w), [r.get("ref") for r in w.findall("nd") if r.get("ref") in nodes]) for w in root.iter("way")}
    line = lambda refs: [nodes[r] for r in refs]

    # Layers.
    parks, water, major, minor, paths, rail = [], [], [], [], [], []
    for g, refs in ways.values():
        pts = line(refs)
        if len(pts) < 2 or not in_view(pts):
            continue
        hw = g.get("highway")
        if g.get("leisure") == "park":
            parks.append(pts)
        elif g.get("natural") == "water" or g.get("water"):
            water.append(pts)
        elif g.get("railway") == "rail":
            rail.append(pts)
        elif hw in MAJOR:
            major.append(pts)
        elif hw in MINOR:
            minor.append(pts)
        elif hw in PATHS:
            paths.append(pts)
    # Multipolygons (the pond, the park): their outer ways, joined end to end.
    for rel in root.iter("relation"):
        g = tags(rel)
        layer = parks if g.get("leisure") == "park" else water if g.get("natural") == "water" else None
        if layer is None:
            continue
        rings, pending = [], [line(ways[m.get("ref")][1]) for m in rel.findall("member")
                              if m.get("type") == "way" and m.get("role") == "outer" and m.get("ref") in ways]
        while pending:
            ring = pending.pop(0)
            grown = True
            while grown and ring[0] != ring[-1]:
                grown = False
                for i, seg in enumerate(pending):
                    if seg and seg[0] == ring[-1]:
                        ring += seg[1:]
                    elif seg and seg[-1] == ring[-1]:
                        ring += seg[::-1][1:]
                    else:
                        continue
                    pending.pop(i)
                    grown = True
                    break
            if len(ring) > 2 and in_view(ring):
                rings.append(ring)
        layer.extend(rings)

    # The walk: Dijkstra over walkable ways.
    graph: dict[str, list[tuple[str, float]]] = {}
    def dist(a, b):
        (ax, ay), (bx, by) = xy(nodes[a]), xy(nodes[b])
        return math.hypot(bx - ax, by - ay)
    for g, refs in ways.values():
        if g.get("highway") in WALK and g.get("access") != "private":
            for a, b in zip(refs, refs[1:]):
                d = dist(a, b)
                graph.setdefault(a, []).append((b, d))
                graph.setdefault(b, []).append((a, d))
    def nearest(p):
        return min(graph, key=lambda n: (nodes[n][0] - p[0]) ** 2 + ((nodes[n][1] - p[1]) * COS) ** 2)
    def route(a, b):
        best, prev, queue = {a: 0.0}, {}, [(0.0, a)]
        while queue:
            d, u = heapq.heappop(queue)
            if u == b:
                break
            if d > best[u]:
                continue
            for v, w in graph[u]:
                if d + w < best.get(v, math.inf):
                    best[v], prev[v] = d + w, u
                    heapq.heappush(queue, (d + w, v))
        path = [b]
        while path[-1] != a:
            path.append(prev[path[-1]])
        return path[::-1]
    start, cafe = nearest(START), nearest(CAFE)
    bridge = max(ways[BRIDGE_WAY][1], key=lambda r: nodes[r][0])
    stops_along = [start] + [nearest(p) for p in AROUND_POND] + [bridge, cafe]
    walk = [start]
    for a, b in zip(stops_along, stops_along[1:]):
        walk += route(a, b)[1:]
    # A turning point can snap to the end of a short spur, which the walk
    # would go up and straight back down: cut those out-and-backs.
    trimmed = []
    for n in walk:
        if len(trimmed) >= 2 and trimmed[-2] == n:
            trimmed.pop()
        else:
            trimmed.append(n)
    walk = trimmed
    walk_xy = simplify([xy(nodes[n]) for n in walk], 1.0)
    # How far along the walk each stop is (0 to 1), so it pops in as the line reaches it.
    lengths = [math.hypot(b[0] - a[0], b[1] - a[1]) for a, b in zip(walk_xy, walk_xy[1:])]
    total = sum(lengths)
    def frac(n):
        p = xy(nodes[n])
        k = min(range(len(walk_xy)), key=lambda i: math.hypot(walk_xy[i][0] - p[0], walk_xy[i][1] - p[1]))
        return sum(lengths[:k]) / total
    stops = [(xy(nodes[n]), frac(n)) for n in (start, bridge, cafe)]

    w, h = xy((SOUTH, EAST))
    route_d = "M" + " ".join(f"{x:.0f} {y:.0f}" for x, y in walk_xy)
    sx, sy = xy(nodes[start])
    px, py = xy((35.69935, 139.5752))  # in the pond's western arm, clear of the route
    svg = f'''<svg class="map" viewBox="0 0 {w:.0f} {h:.0f}" role="img" aria-label="Map of the walk: from Kichijoji Station's park exit down to the east end of Inokashira Pond, once around the pond to Nanai Bridge, then up Nanaibashi-dori to the café. About {total / 1000:.1f} km.">
          <path class="park" d="{d_attr(parks, True)}"/>
          <path class="water" d="{d_attr(water, True)}"/>
          <path class="paths" d="{d_attr(paths)}"/>
          <path class="minor" d="{d_attr(minor)}"/>
          <path class="major" d="{d_attr(major)}"/>
          <path class="rail" d="{d_attr(rail)}"/>
          <text class="label" x="{sx + 14:.0f}" y="{sy - 18:.0f}">Kichijoji Sta.</text>
          <text class="label water-label" x="{px:.0f}" y="{py:.0f}">Inokashira Pond</text>
          <path class="route-track" d="{route_d}" pathLength="1"/>
          <path class="route" id="route" d="{route_d}" pathLength="1"/>
          {"".join(f'<circle class="stop" cx="{x:.0f}" cy="{y:.0f}" r="9" style="--at: {f:.2f}"/>' for (x, y), f in stops)}
          <circle class="walker-dot" r="8"/>
        </svg>'''
    html = SITE.read_text(encoding="utf-8")
    html, n = re.subn(r"(<!-- map:start -->).*?(<!-- map:end -->)", lambda m: m.group(1) + "\n        " + svg + "\n        " + m.group(2), html, flags=re.S)
    if n != 1:
        sys.exit("site/index.html needs one <!-- map:start --> ... <!-- map:end --> pair")
    SITE.write_text(html, encoding="utf-8")
    print(f"map: {total:.0f} m walk, stops at {[round(f, 2) for _, f in stops]}, {len(svg) / 1024:.1f} KB")


if __name__ == "__main__":
    main()
