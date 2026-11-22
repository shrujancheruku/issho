# Browsers reach the API on their own origin, and the API has no CORS

The Go API answers no CORS preflight and sends no `Access-Control-*` header, so a browser lets no page on another origin read what it answers. The web and the admin each live on their own name (`dev.joinissho.com`, `admin-dev.joinissho.com`; the API is `api-dev.joinissho.com`), so each forwards the API's paths, `/issho.v1.*` and nothing else, from its own origin. The Expo server does it in `apps/web/api-proxy.js`, and the admin's Cloudflare Worker in `apps/admin/worker/index.ts`. The admin's page calls relative paths, as it already does through Vite's proxy in development; the web's is built with its own origin as the API's address. Phones are not browsers and call the API's name directly. Both forwarders drop the headers that carry the member's address ([ADR 0005](0005-no-phone-number-and-no-ip-address.md)), so the API sees the forwarder's address instead. This was decided in [Dev deploys on every merge to main](https://github.com/shrujancheruku/issho/issues/605), and [Deploying dev](../deploy.md#how-the-browser-reaches-the-api) has the details.

We chose this because CORS would cost more than the hop does:

- **No list of allowed origins to keep.** Each environment and each new page adds a name. A list that falls behind breaks a page, and a list loosened to fix one (a wildcard, a reflected `Origin`) opens the API to every site. With no CORS there is nothing to get wrong.
- **No preflight.** A ConnectRPC call is a `POST` with `application/json` and an `Authorization` header, so a browser would send an `OPTIONS` before it, once per path until its cache expires. From Vietnam to the VM in Singapore, that is a round trip on the first call to each procedure.
- **The same paths in development and deployed**, for the admin: its page has one way to reach the API, and only the server in front of it changes.

What it protects is narrower than it sounds. Sign-in is a bearer token, not a cookie, so another site's script could not act as a member through CORS anyway. CORS is not access control either: anything that is not a browser can call the API's name.

## Considered options

- **CORS on the API with an allowed-origin list** (`dev`, `admin-dev`, and later production's names). Rejected for the list and the preflight above. It would have saved both forwarders.
- **The web and the API on one name, routed by path in kamal-proxy** (`/` to the Expo server, `/issho.v1.` to the API). Tried first and refused by kamal-proxy, which wants a name's TLS settings on the root path's service. It would have needed no forwarder in the Expo server.
- **A Cloudflare Worker in front of the web's name, routing by path.** Not taken. It is the same forwarding at the edge, but it costs a Worker request for every page view, not just every RPC, and it adds a third thing to deploy for the web.

## Consequences

- Every browser call takes a second hop. The web's goes from the browser through Cloudflare to the Expo server, then back through Cloudflare to kamal-proxy and the API. The admin's goes through its Worker, which counts each RPC against Cloudflare's Worker requests. On dev's 1 GB VM, the Expo server carries the web's API traffic as well as its pages.
- Two forwarders must stay correct: only the API's paths, the method, headers and body passed on, answers streamed back, the member's address dropped, and 502 when the API cannot be reached. Their tests (`apps/web/api-proxy.test.js`, `apps/admin/worker/index.test.ts`) check each of these, and a new page on a new name needs a forwarder of its own.
- Production's web and admin forward to production's API the same way.
- The member app's web under `expo start` (port 8081) calls `EXPO_PUBLIC_API_URL` on another origin, so in a browser it cannot read a local API. The Expo server built with `API_ORIGIN` set does reach it, as on dev. Phones and simulators are not affected.
- Don't add `Access-Control-Allow-Origin` to the API to fix a page that "can't reach the API". Forward the API's paths from that page's origin instead.
- **What would reverse this:** a browser page that has to call the API from an origin that cannot forward. Examples are a page on a static host with no server or Worker, an embed on someone else's site, or a browser extension. That needs a CORS rule in the API with an explicit origin list, written in its own ticket, and an ADR that replaces this one. If the hop's latency or the Expo server's load shows in the web's numbers, the forwarding can move to the edge or onto one name. The page would still call its own origin, so this decision would stand.
