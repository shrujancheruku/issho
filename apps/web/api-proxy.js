// Forwards the API's paths to the API, so a page calls it on its own origin. The
// API has no CORS, by design, so a browser can only call it from where it is
// proxied: kamal-proxy cannot do it on dev's shared name (it wants TLS on the
// root path's service), and the admin's Worker does the same for its own page
// (apps/admin/worker/index.ts). Only paths of the API's services (`/issho.v1.`)
// are forwarded, so this is not an open proxy.
const { Readable } = require('node:stream');

// Not copied between the two sides: they describe one connection, not the message.
const HOP = new Set(['connection', 'keep-alive', 'proxy-connection', 'transfer-encoding', 'upgrade', 'te', 'trailer', 'host']);
// fetch has already decoded the body, so the encoding and length no longer fit it.
const DECODED = new Set(['content-encoding', 'content-length']);

function apiProxy(apiOrigin) {
  return async (req, res, next) => {
    if (!req.url.startsWith('/issho.v1.')) return next();
    const headers = new Headers();
    for (const [name, value] of Object.entries(req.headers)) {
      if (!HOP.has(name)) headers.set(name, Array.isArray(value) ? value.join(', ') : value);
    }
    const hasBody = req.method !== 'GET' && req.method !== 'HEAD';
    try {
      const upstream = await fetch(new URL(req.url, apiOrigin), {
        method: req.method,
        headers,
        body: hasBody ? Readable.toWeb(req) : undefined,
        duplex: 'half',
        // The API's redirects are the browser's to follow, not ours.
        redirect: 'manual',
      });
      res.status(upstream.status);
      upstream.headers.forEach((value, name) => {
        if (!HOP.has(name) && !DECODED.has(name)) res.setHeader(name, value);
      });
      if (!upstream.body) return res.end();
      Readable.fromWeb(upstream.body).on('error', () => res.destroy()).pipe(res);
    } catch {
      if (!res.headersSent) res.status(502).type('text/plain').send('the API cannot be reached');
      else res.destroy();
    }
  };
}

module.exports = { apiProxy };
