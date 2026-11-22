// The admin's Cloudflare Worker. The static files are served by Cloudflare's
// asset handling without waking this code (`run_worker_first` in wrangler.jsonc
// lists only the API's paths), so all it does is hand an RPC to the API.
//
// Why a proxy: the admin and the API are on different addresses (the spec is
// silent on how they meet), and the API has no CORS, by design: no browser
// origin is trusted to call it cross-origin. Forwarding on the admin's own
// origin keeps it that way and matches Vite's dev proxy, so the page calls the
// same relative paths in development and deployed. The trade-off is ADR 0006
// (docs/adr/0006-browsers-reach-the-api-on-their-own-origin.md).

export interface Env {
  /** The API's origin, e.g. https://api-dev.joinissho.com (wrangler.jsonc `vars`). */
  API_ORIGIN: string;
}

/**
 * The member's address, and where Cloudflare places it, which Cloudflare adds on
 * the way here: not passed on, since Issho keeps no IP address anywhere
 * (docs/spec/privacy.md). The Expo server's forwarder drops the same
 * (apps/web/api-proxy.js).
 */
const CLIENT_ADDRESS = [
  'cf-connecting-ip', 'cf-connecting-ipv6', 'cf-pseudo-ipv4', 'cf-ipcountry', 'x-forwarded-for', 'x-real-ip',
  'forwarded', 'true-client-ip', 'x-client-ip', 'x-cluster-client-ip', 'client-ip',
];

/** Every ConnectRPC procedure's path starts with its proto package (`issho.v1.`). */
const API_PREFIX = '/issho.v1.';

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    // Anything but an RPC is not this Worker's to forward, whatever reached it.
    if (!url.pathname.startsWith(API_PREFIX)) return new Response('Not found', { status: 404 });

    const headers = new Headers(request.headers);
    for (const name of CLIENT_ADDRESS) headers.delete(name);
    try {
      return await fetch(`${env.API_ORIGIN}${url.pathname}${url.search}`, {
        method: request.method,
        headers,
        body: request.body,
        // The API never redirects; one that did would be handed to the browser as is.
        redirect: 'manual',
      });
    } catch {
      return new Response('The API could not be reached.', { status: 502 });
    }
  },
};
