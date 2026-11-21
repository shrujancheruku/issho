// The admin's Cloudflare Worker. The static files are served by Cloudflare's
// asset handling without waking this code (`run_worker_first` in wrangler.jsonc
// lists only the API's paths), so all it does is hand an RPC to the API.
//
// Why a proxy: the admin and the API are on different addresses (the spec is
// silent on how they meet), and the API has no CORS, by design: no browser
// origin is trusted to call it cross-origin. Forwarding on the admin's own
// origin keeps it that way and matches Vite's dev proxy, so the page calls the
// same relative paths in development and deployed. docs/deploy.md has the
// trade-off.

export interface Env {
  /** The API's origin, e.g. https://api-dev.joinissho.com (wrangler.jsonc `vars`). */
  API_ORIGIN: string;
}

/** Every ConnectRPC procedure's path starts with its proto package (`issho.v1.`). */
const API_PREFIX = '/issho.v1.';

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    // Anything but an RPC is not this Worker's to forward, whatever reached it.
    if (!url.pathname.startsWith(API_PREFIX)) return new Response('Not found', { status: 404 });

    try {
      return await fetch(`${env.API_ORIGIN}${url.pathname}${url.search}`, {
        method: request.method,
        headers: request.headers,
        body: request.body,
        // The API never redirects; one that did would be handed to the browser as is.
        redirect: 'manual',
      });
    } catch {
      return new Response('The API could not be reached.', { status: 502 });
    }
  },
};
