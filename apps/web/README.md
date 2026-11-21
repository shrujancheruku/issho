# @issho/web

What serves the member app's web build in production: a small Express server (`server.js`) over the files `expo export --platform web` writes for `apps/app` (`web.output: "server"`). Static files come from `dist/client`; every other request goes to Expo's request handler (`dist/server`), which renders the page on the server. It holds no tokens, cookies or personal data, so what it sends can be cached ([The web at one address](../../docs/spec/architecture.md#the-web-at-one-address)).

`GET /up` answers 200 before anything else. It is the probe kamal-proxy polls ([docs/deploy.md](../../docs/deploy.md)).

## The image

`Dockerfile` builds from the repository root, in two stages: the app's export, then a runtime that installs only `express` and `expo-server` and copies the export in as `dist`.

```sh
docker build --file apps/web/Dockerfile --build-arg EXPO_PUBLIC_API_URL=https://api-dev.joinissho.com .
docker run --publish 8081:8081 <image>      # PORT overrides 8081
```

`EXPO_PUBLIC_API_URL` is the Go API's address. Like every `EXPO_PUBLIC_*` value it is bundled into the app, so it is public and is a build argument, not a secret. It differs per environment, so each environment builds its own image.

`Dockerfile.dockerignore` keeps the build context to the files the Dockerfile copies.

## Testing

`make image` (also `scripts/image.sh`) builds this image, runs it, and checks that `/up` answers 200, that `/plans` holds its words (`No plans yet.`) in the HTML a client with no JavaScript gets, and that the script the page names is served. It needs Docker, and it removes what it started. There are no unit tests: the file is Expo's own recipe, and what can go wrong is the image, which this checks.

`expo-server`'s version must stay the one `expo` depends on (`apps/app`'s lockfile entry): the server bundle's format is theirs. `package.json` pins its minor.
