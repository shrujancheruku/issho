# @issho/web

What serves the member app's web build in production: a small Express server (`server.js`) over the files `expo export --platform web` writes for `apps/app` (`web.output: "server"`). Static files come from `dist/client`; every other request goes to Expo's request handler (`dist/server`), which renders the page on the server. It holds no tokens, cookies or personal data, so what it sends can be cached ([The web at one address](../../docs/spec/architecture.md#the-web-at-one-address)).

`GET /up` answers 200 before anything else. It is the probe kamal-proxy polls ([docs/deploy.md](../../docs/deploy.md)).

With `API_ORIGIN` set, `api-proxy.js` forwards the API's paths (`/issho.v1.*`, and nothing else) to it, with the request's method, headers and body and the API's answer as it is, so the page calls the API on its own origin. The headers that carry the member's address (`cf-connecting-ip`, `x-forwarded-for` and the like) are dropped, since Issho keeps no IP address. The API has no CORS, by design ([ADR 0006](../../docs/adr/0006-browsers-reach-the-api-on-their-own-origin.md)), so this is how a browser reaches it. Unset (the image check, local runs), nothing is forwarded. Dev sets it to `https://api-dev.joinissho.com` in `config/deploy.web.dev.yml`.

## The image

`Dockerfile` builds from the repository root, in two stages: the app's export, then a runtime that installs only `express` and `expo-server` and copies the export in as `dist`.

```sh
EXPO_PUBLIC_SUPABASE_URL=<project url> EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY=<publishable key> \
docker build --file apps/web/Dockerfile --build-arg EXPO_PUBLIC_API_URL=https://dev.joinissho.com \
  --build-arg SUPABASE_VALUES_HASH=<any string that changes with the two values> \
  --secret id=EXPO_PUBLIC_SUPABASE_URL,env=EXPO_PUBLIC_SUPABASE_URL \
  --secret id=EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY,env=EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY .
docker run --publish 8081:8081 <image>      # PORT overrides 8081
```

`EXPO_PUBLIC_API_URL` is the address the page calls the API at: the web's own origin, since the API has no CORS and the server forwards its paths (`API_ORIGIN`, above; [ADR 0006](../../docs/adr/0006-browsers-reach-the-api-on-their-own-origin.md)). Pointed at the API's own name, the page's calls would be cross-origin and refused. Like every `EXPO_PUBLIC_*` value it is bundled into the app, so it is public and is a build argument, not a secret. It differs per environment, so each environment builds its own image.

The sign-in's two values (`EXPO_PUBLIC_SUPABASE_URL`, `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY`) are build secrets, so a plain `docker build` needs them as `--secret` (above); an empty or missing one fails the build. They end up in the bundle (public by design) but not in the repository, a build argument or the image's config and history. `SUPABASE_VALUES_HASH` exists for BuildKit's cache, which ignores secret mounts: without it a rebuild with new values could reuse the old bundle.

`Dockerfile.dockerignore` keeps the build context to the files the Dockerfile copies.

## Testing

`make image` (also `scripts/image.sh`) builds this image, runs it, and checks that `/up` answers 200, that `/plans` holds its words (`No plans yet.`) in the HTML a client with no JavaScript gets, and that the script the page names is served. It also checks that the bundle holds the stand-in Supabase URL and key, that a second build with other values on the same daemon bundles the new ones (not the cached first), and that an empty value fails the build. It needs Docker, and it removes what it started. `api-proxy.test.js` (`pnpm --filter @issho/web test`, run by `make deploy-test`, so by `make test`) tests the forwarder against a stand-in API on a real port: an RPC is forwarded with its method, headers and body; the API's errors pass through; a path outside the API is the page and never reaches the API; the member's address headers are not passed on; an unreachable API answers 502. The rest of the file is Expo's own recipe, and what can go wrong there is the image, which `make image` checks.

`expo-server`'s version must stay the one `expo` depends on (`apps/app`'s lockfile entry): the server bundle's format is theirs. `package.json` pins its minor.

## Logs

The server logs only its startup line. [Technical operations](../../docs/spec/operations.md#errors-and-logs) says it logs the route (and the account, once there is one); that waits for the sign-in work, which gives the web an account to log. Then it adds one line per request: route, status and duration, never the visitor's address or the query.
