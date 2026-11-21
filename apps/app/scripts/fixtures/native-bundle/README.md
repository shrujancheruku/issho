# native-bundle fixture

`web-only.web.ts` exists only for the web, so Metro resolves an import of it on the web and fails on iOS and Android. `route.tsx` imports it. `scripts/native-bundle.test.sh` copies `route.tsx` into `src/app/` for one run, expects `scripts/native-bundle.sh` to fail with the resolver's error for `web-only`, and removes the copy on exit. Nothing here is a real route or is bundled by the app.
