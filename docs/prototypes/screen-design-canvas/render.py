#!/usr/bin/env python3
"""Render every board of a Design canvas to PNGs and report layout problems.

    python3 render.py <root> [--out DIR] [--only NAME ...] [--themes light,dark]

<root> is a folder holding the canvas's files at their canvas paths
(<root>/project/canvas.json, <root>/project/*.dc.html, <root>/project/ds/...),
as an agent saves them with the Artifact tool's read action. Each board is
served over a local HTTP server with support.js (this folder's stand-in for
the canvas runtime) beside it, opened in headless Chrome at its frame's size
once per theme (?theme=light, ?theme=dark), screenshotted, and checked.

A Vietnamese board is its own file (<Name>-vi.dc.html), so it is rendered in
both themes like any other.

Output: <out>/<board>.<theme>.png and <out>/report.txt. Exit status 1 when a
board has problems, so a session can't miss them. Read every PNG anyway: the
checks catch geometry, not sense.
"""
import argparse
import functools
import http.server
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading
from concurrent.futures import ThreadPoolExecutor
from html import unescape

CHROME = os.environ.get('CHROME', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome')
HERE = os.path.dirname(os.path.abspath(__file__))


def serve(directory):
    handler = functools.partial(QuietHandler, directory=directory)
    httpd = http.server.ThreadingHTTPServer(('127.0.0.1', 0), handler)
    threading.Thread(target=httpd.serve_forever, daemon=True).start()
    return httpd


class QuietHandler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, *args):
        pass


def chrome(args):
    profile = tempfile.mkdtemp(prefix='dc-chrome-')
    try:
        return subprocess.run(
            [CHROME, '--headless=new', '--disable-gpu', '--hide-scrollbars', '--no-first-run',
             '--no-default-browser-check', '--user-data-dir=' + profile, '--virtual-time-budget=8000',
             *args],
            capture_output=True, text=True, timeout=90)
    finally:
        shutil.rmtree(profile, ignore_errors=True)


def render(url, w, h, png):
    chrome(['--window-size=%d,%d' % (w, h), '--screenshot=' + png, url])
    dom = chrome(['--window-size=%d,%d' % (w, h), '--dump-dom', url]).stdout
    found = []
    m = re.search(r'<meta name="dc-check" content="([^"]*)"', dom)
    if m:
        found = json.loads(unescape(m.group(1)))
    else:
        found = ['the stand-in runtime never finished rendering']
    e = re.search(r'<meta name="dc-error" content="([^"]*)"', dom)
    if e:
        found.insert(0, 'script error: ' + unescape(e.group(1)))
    return found


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('root')
    ap.add_argument('--out', default=None)
    ap.add_argument('--only', nargs='*', help='board file names (with or without .dc.html)')
    ap.add_argument('--themes', default='light,dark')
    a = ap.parse_args()

    project = os.path.join(os.path.abspath(a.root), 'project')
    index = json.load(open(os.path.join(project, 'canvas.json')))
    out = os.path.abspath(a.out or os.path.join(a.root, 'shots'))
    os.makedirs(out, exist_ok=True)

    # Serve a copy so the stand-in support.js never lands in the canvas folder.
    build = tempfile.mkdtemp(prefix='dc-build-')
    shutil.copytree(project, build, dirs_exist_ok=True)
    shutil.copy(os.path.join(HERE, 'support.js'), os.path.join(build, 'support.js'))
    httpd = serve(build)
    base = 'http://127.0.0.1:%d/' % httpd.server_address[1]

    only = {o if o.endswith('.dc.html') else o + '.dc.html' for o in (a.only or [])}
    jobs = []
    for name, frame in index['boards'].items():
        if only and name not in only:
            continue
        if not os.path.exists(os.path.join(project, name)):
            continue
        # A -dark board shows another board dark; rendering that board in dark covers it.
        if name.endswith('-dark.dc.html') and not only:
            continue
        stem = name[:-len('.dc.html')]
        for theme in a.themes.split(','):
            png = os.path.join(out, '%s.%s.png' % (stem.replace('/', '__'), theme))
            jobs.append((name, theme, base + name + '?theme=' + theme, int(frame['w']), int(frame['h']), png))

    with ThreadPoolExecutor(max_workers=6) as pool:
        results = list(pool.map(lambda j: (j, render(*j[2:])), jobs))
    httpd.shutdown()
    shutil.rmtree(build, ignore_errors=True)

    lines, bad = [], 0
    for (name, theme, _, w, h, png), problems in results:
        status = 'ok' if not problems else '%d problem(s)' % len(problems)
        lines.append('%s [%s] %dx%d: %s' % (name, theme, w, h, status))
        lines += ['    - ' + p for p in problems]
        bad += bool(problems)
    report = '\n'.join(lines) + '\n'
    open(os.path.join(out, 'report.txt'), 'w').write(report)
    sys.stdout.write(report)
    sys.stdout.write('\n%d renders, %d with problems. PNGs in %s\n' % (len(results), bad, out))
    sys.exit(1 if bad else 0)


if __name__ == '__main__':
    main()
