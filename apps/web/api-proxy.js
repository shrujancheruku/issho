// Forwards the API's paths to the API, so a page calls it on its own origin. The
// API has no CORS, by design, so a browser can only call it from where it is
// proxied: kamal-proxy cannot do it on dev's shared name (it wants TLS on the
// root path's service), and the admin's Worker does the same for its own page
// (apps/admin/worker/index.ts). Only paths of the API's services (`/issho.v1.`)
// are forwarded, so this is not an open proxy. Why not CORS: ADR 0006
// (docs/adr/0006-browsers-reach-the-api-on-their-own-origin.md).
const { Readable } = require('node:stream');

// Not copied between the two sides: they describe one connection, not the message.
const HOP = new Set(['connection', 'keep-alive', 'proxy-connection', 'transfer-encoding', 'upgrade', 'te', 'trailer', 'host']);
// The member's address, and where Cloudflare places it, which Cloudflare and
// kamal-proxy add on the way here: not passed on, since Issho keeps no IP address
// anywhere (docs/spec/privacy.md). The API sees this server's address instead.
// The admin's Worker drops the same (apps/admin/worker/index.ts); a test there
// fails if the two lists differ.
const CLIENT_ADDRESS = new Set([
  'cf-connecting-ip', 'cf-connecting-ipv6', 'cf-pseudo-ipv4', 'cf-ipcountry', 'x-forwarded-for', 'x-real-ip',
  'forwarded', 'true-client-ip', 'x-client-ip', 'x-cluster-client-ip', 'client-ip',
]);
// fetch has already decoded the body, so the encoding and length no longer fit it.
const DECODED = new Set(['content-encoding', 'content-length']);

function apiProxy(apiOrigin) {
  return async (req, res, next) => {
    // Test the path the request will actually reach: `new URL` resolves `..`, so
    // `/issho.v1./../up` would pass a check on the raw string and land on `/up`.
    // The origin is checked too, since `//host/..` resolves to another host.
    const target = new URL(req.url, apiOrigin);
    // An encoded slash, backslash or dot is no part of an RPC's name; an API that
    // decodes it would let `..%2f` climb out after all.
    if (
      target.origin !== new URL(apiOrigin).origin ||
      !target.pathname.startsWith('/issho.v1.') ||
      /%(2e|2f|5c)/i.test(target.pathname)
    ) {
      return next();
    }
    const headers = new Headers();
    for (const [name, value] of Object.entries(req.headers)) {
      if (!HOP.has(name) && !CLIENT_ADDRESS.has(name)) headers.set(name, Array.isArray(value) ? value.join(', ') : value);
    }
    const hasBody = req.method !== 'GET' && req.method !== 'HEAD';
    try {
      const upstream = await fetch(target, {
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

module.exports = { apiProxy, CLIENT_ADDRESS };
