// Serves the member app's web build: the static files of `dist/client` and, for
// everything else, Expo's request handler, which renders the page on the server
// (`web.output: "server"`). This is Expo's own Express recipe; the Expo server
// holds no tokens, cookies or personal data, so what it sends can be cached.
//
// With API_ORIGIN set, the API's paths are forwarded to it (api-proxy.js), so the
// page calls the API on its own origin.
//
// The build is `expo export -p web` in apps/app, copied next to this file as
// `dist` by apps/web/Dockerfile. DIST overrides where to look.
const path = require('node:path');
const express = require('express');
const { createRequestHandler } = require('expo-server/adapter/express');
const { apiProxy } = require('./api-proxy');

const dist = path.resolve(process.env.DIST ?? path.join(__dirname, 'dist'));
const port = Number(process.env.PORT ?? 8081);

const app = express();
app.disable('x-powered-by');

// The deploy's liveness probe (kamal-proxy polls a plain GET), served before
// anything else so it says nothing about a page.
app.get('/up', (_req, res) => res.sendStatus(200));

if (process.env.API_ORIGIN) app.use(apiProxy(process.env.API_ORIGIN));

// Expo hashes the names of what is under /_expo and /assets, so they never change.
app.use(
  express.static(path.join(dist, 'client'), {
    extensions: ['html'],
    setHeaders: (res, file) => {
      if (/[\\/](_expo|assets)[\\/]/.test(file)) res.setHeader('Cache-Control', 'public, max-age=31536000, immutable');
    },
  }),
);
app.all('/{*splat}', createRequestHandler({ build: path.join(dist, 'server') }));

// No request log yet: operations.md says this server logs the route (and the
// account once there is one), and there is no account until the sign-in work
// gives the web one. When it does, add one line per request there (route,
// status, duration; never the address or a query), not before.
app.listen(port, '0.0.0.0', () => console.log(`web: listening on :${port}`));
