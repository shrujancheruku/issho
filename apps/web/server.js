// Serves the member app's web build: the static files of `dist/client` and, for
// everything else, Expo's request handler, which renders the page on the server
// (`web.output: "server"`). This is Expo's own Express recipe; the Expo server
// holds no tokens, cookies or personal data, so what it sends can be cached.
//
// The build is `expo export -p web` in apps/app, copied next to this file as
// `dist` by apps/web/Dockerfile. DIST overrides where to look.
const path = require('node:path');
const express = require('express');
const { createRequestHandler } = require('expo-server/adapter/express');

const dist = path.resolve(process.env.DIST ?? path.join(__dirname, 'dist'));
const port = Number(process.env.PORT ?? 8081);

const app = express();
app.disable('x-powered-by');

// The deploy's liveness probe (kamal-proxy polls a plain GET), served before
// anything else so it says nothing about a page.
app.get('/up', (_req, res) => res.sendStatus(200));

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

app.listen(port, '0.0.0.0', () => console.log(`web: listening on :${port}`));
