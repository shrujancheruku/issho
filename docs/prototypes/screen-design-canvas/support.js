/*
 * A local stand-in for the Design canvas runtime, so boards can be rendered
 * and screenshotted outside claude.ai. It covers what this canvas's boards
 * use: {{holes}}, <sc-if>, <sc-for>, <dc-import>, events, setState and the
 * <helmet>. It is not the real runtime: anything it can't render shows up as
 * a broken screenshot, which is the point of looking at every one.
 *
 * Props come from the URL query (?theme=dark&lang=vi), parsed as JSON where
 * they parse. After rendering, it measures the board and writes what it found
 * to <meta name="dc-check"> for render.py to read.
 */
(function () {
  'use strict';

  class DCLogic {
    constructor(props) {
      this.props = props;
      this.state = {};
    }
    setState(patch) {
      const p = typeof patch === 'function' ? patch(this.state, this.props) : patch;
      this.state = Object.assign({}, this.state, p);
      if (this.__rerender) this.__rerender();
    }
    forceUpdate() {
      if (this.__rerender) this.__rerender();
    }
  }
  window.DCLogic = DCLogic;

  const HOLE = /\{\{\s*([^}]+?)\s*\}\}/g;
  const WHOLE = /^\s*\{\{\s*([^}]+?)\s*\}\}\s*$/;

  function lookup(scope, path) {
    if (/^(true|false|null|-?\d+(\.\d+)?)$/.test(path)) return JSON.parse(path);
    if (/^'.*'$|^".*"$/.test(path)) return path.slice(1, -1);
    return path.split('.').reduce((o, k) => (o == null ? undefined : o[k]), scope);
  }
  function interp(str, scope) {
    const whole = str.match(WHOLE);
    if (whole) return { raw: true, v: lookup(scope, whole[1]) };
    return {
      raw: false,
      v: str.replace(HOLE, (_, p) => {
        const v = lookup(scope, p);
        return v == null ? '' : String(v);
      }),
    };
  }
  const camel = (s) => s.replace(/-([a-z])/g, (_, c) => c.toUpperCase());

  const helmetsAdded = new Set();
  function addHelmet(key, helmet) {
    if (!helmet || helmetsAdded.has(key)) return;
    helmetsAdded.add(key);
    for (const n of Array.from(helmet.childNodes)) document.head.appendChild(document.importNode(n, true));
  }

  // A component is a parsed .dc.html: its template nodes, helmet and class.
  function parseComponent(doc, key) {
    const xdc = doc.querySelector('x-dc');
    const script = doc.querySelector('script[data-dc-script]');
    const helmet = xdc.querySelector(':scope > helmet');
    const template = Array.from(xdc.childNodes).filter((n) => n !== helmet);
    let Cls = class extends DCLogic { renderVals() { return {}; } };
    if (script) {
      Cls = new Function('DCLogic', script.textContent + '\n;return Component;')(DCLogic);
    }
    return { key, xdc, helmet, template, Cls };
  }

  const cache = {};
  function loadImport(name) {
    if (cache[name]) return cache[name];
    const xhr = new XMLHttpRequest();
    xhr.open('GET', name + '.dc.html', false);
    xhr.send();
    if (xhr.status !== 200) throw new Error('dc-import: cannot load ' + name);
    const doc = new DOMParser().parseFromString(xhr.responseText, 'text/html');
    for (const link of doc.head.querySelectorAll('link[rel=stylesheet]')) {
      if (!document.head.querySelector('link[href="' + link.getAttribute('href') + '"]')) {
        document.head.appendChild(document.importNode(link, true));
      }
    }
    return (cache[name] = parseComponent(doc, name));
  }

  function mount(comp, props, container) {
    addHelmet(comp.key, comp.helmet);
    const inst = new comp.Cls(props);
    const draw = () => {
      const scope = inst.renderVals() || {};
      container.replaceChildren(...comp.template.map((n) => renderNode(n, scope)).filter(Boolean));
    };
    inst.__rerender = draw;
    draw();
    if (inst.componentDidMount) inst.componentDidMount();
  }

  function renderChildren(node, scope) {
    const frag = document.createDocumentFragment();
    for (const c of Array.from(node.childNodes)) {
      const r = renderNode(c, scope);
      if (r) frag.appendChild(r);
    }
    return frag;
  }

  function renderNode(node, scope) {
    if (node.nodeType === Node.TEXT_NODE) return document.createTextNode(interp(node.nodeValue, scope).v);
    if (node.nodeType !== Node.ELEMENT_NODE) return null;
    const tag = node.localName;
    if (tag === 'helmet') return null;
    if (tag === 'sc-if') {
      return interp(node.getAttribute('value') || '', scope).v ? renderChildren(node, scope) : null;
    }
    if (tag === 'sc-for') {
      const list = interp(node.getAttribute('list') || '', scope).v || [];
      const as = node.getAttribute('as') || 'item';
      const frag = document.createDocumentFragment();
      list.forEach((item, i) => {
        const s = Object.create(scope);
        s[as] = item;
        s.$index = i;
        frag.appendChild(renderChildren(node, s));
      });
      return frag;
    }
    if (tag === 'dc-import') {
      const comp = loadImport(node.getAttribute('name'));
      const props = {};
      for (const a of Array.from(node.attributes)) {
        if (a.name === 'name' || a.name.startsWith('hint-')) continue;
        props[camel(a.name)] = interp(a.value, scope).v;
      }
      const box = document.createElement('dc-import');
      box.style.display = 'contents';
      mount(comp, props, box);
      return box;
    }
    const el = document.createElementNS(node.namespaceURI, tag);
    for (const a of Array.from(node.attributes)) {
      const r = interp(a.value, scope);
      if (/^on[A-Z]/.test(a.name) || (/^on[a-z]+$/.test(a.name) && typeof r.v === 'function')) {
        if (typeof r.v === 'function') el.addEventListener(a.name.slice(2).toLowerCase(), r.v);
        continue;
      }
      if (r.raw && (r.v === false || r.v == null)) continue;
      el.setAttribute(a.name, r.raw && r.v === true ? '' : String(r.v));
    }
    el.appendChild(renderChildren(node, scope));
    return el;
  }

  // What render.py reports: things the owner caught by eye on v1's canvas.
  function check(root) {
    const problems = [];
    const rr = root.getBoundingClientRect();
    const label = (el) => {
      const t = (el.getAttribute('aria-label') || el.textContent || '').trim().replace(/\s+/g, ' ');
      return el.localName + (t ? ' "' + t.slice(0, 40) + '"' : '');
    };
    const clippedBelowRoot = (el) => {
      for (let p = el.parentElement; p && p !== root; p = p.parentElement) {
        const cs = getComputedStyle(p);
        if (cs.overflowX !== 'visible' || cs.overflowY !== 'visible') return true;
      }
      return false;
    };
    // Fixed bars: absolutely placed blocks pinned to the top or bottom edge.
    const bars = Array.from(root.querySelectorAll('*')).filter((el) => {
      const cs = getComputedStyle(el);
      if (cs.position !== 'absolute' && cs.position !== 'fixed' && cs.position !== 'sticky') return false;
      const r = el.getBoundingClientRect();
      // A map or backdrop filling the board is not a bar.
      return r.width >= rr.width * 0.9 && r.height < rr.height * 0.4 &&
        (Math.abs(r.bottom - rr.bottom) < 1 || Math.abs(r.top - rr.top) < 1);
    });
    const all = Array.from(root.querySelectorAll('*'));
    for (const el of all) {
      const r = el.getBoundingClientRect();
      if (!r.width || !r.height) continue;
      const cs = getComputedStyle(el);
      if (cs.visibility === 'hidden') continue;
      if ((r.right > rr.right + 0.5 || r.left < rr.left - 0.5) && !clippedBelowRoot(el)) {
        problems.push('off the board sideways: ' + label(el));
      }
      const interactive = el.matches('a, button, input, select, textarea, .is-button');
      const inBar = bars.some((b) => b.contains(el));
      if (interactive && !inBar && !clippedBelowRoot(el) && rr.width <= 500) {
        if (r.left < rr.left + 15.5 || r.right > rr.right - 15.5) problems.push('outside the 16 px gutter: ' + label(el));
      }
      const leaf = el.children.length === 0 && el.textContent.trim();
      if ((leaf || interactive) && !inBar) {
        for (const b of bars) {
          if (b.contains(el) || el.contains(b)) continue;
          const br = b.getBoundingClientRect();
          if (r.bottom > br.top + 0.5 && r.top < br.bottom - 0.5 && r.right > br.left && r.left < br.right) {
            problems.push('under a fixed bar: ' + label(el));
          }
        }
      }
      if (leaf && r.top > rr.bottom - 0.5) problems.push('below the board, cut off: ' + label(el));
    }
    if (root.scrollHeight > root.clientHeight + 1 && getComputedStyle(root).overflowY !== 'visible') {
      problems.push('content taller than the board: ' + root.scrollHeight + ' px in ' + root.clientHeight);
    }
    return Array.from(new Set(problems));
  }

  function boot() {
    const comp = parseComponent(document, '__main__');
    const props = {};
    new URLSearchParams(location.search).forEach((v, k) => {
      try { props[k] = JSON.parse(v); } catch (e) { props[k] = v; }
    });
    const host = comp.xdc;
    host.style.display = 'block';
    mount(comp, props, host);
    let done = false;
    const finish = () => {
      if (done) return;
      done = true;
      // A board that only wraps another (a dark or Vietnamese copy) is checked as the one it wraps.
      let root = host.firstElementChild;
      while (root && root.localName === 'dc-import') root = root.firstElementChild;
      let problems;
      try { problems = root ? check(root) : ['nothing rendered']; } catch (e) { problems = ['check failed: ' + e.message]; }
      const meta = document.createElement('meta');
      meta.name = 'dc-check';
      meta.content = JSON.stringify(problems);
      document.head.appendChild(meta);
      document.documentElement.setAttribute('data-dc-ready', '1');
    };
    // rAF doesn't always fire under headless virtual time; a timeout does.
    (document.fonts ? document.fonts.ready : Promise.resolve()).then(() => setTimeout(finish, 50));
    setTimeout(finish, 4000);
  }

  window.addEventListener('error', (e) => {
    const meta = document.createElement('meta');
    meta.name = 'dc-error';
    meta.content = String(e.message);
    document.head.appendChild(meta);
  });
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
})();
