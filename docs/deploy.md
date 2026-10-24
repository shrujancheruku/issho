# Deploying the backend

The Go API and the River worker run on one VPS, deployed with **Kamal 2** from a single image (`backend/Dockerfile`). Cloudflare sits in front of the API; Supabase (database and auth) stays hosted. The admin SPA and the web build are separate static sites on Cloudflare Workers ([../apps/admin/README.md](../apps/admin/README.md)).

```
app / admin SPA ──HTTPS──▶ Cloudflare ──HTTPS (origin certificate)──▶ kamal-proxy ──▶ api container ─┐
                                                                                      worker container ─┴──▶ Supabase Postgres
```

**Status:** prepared, not deployed. `config/deploy.yml` is checked by Kamal (2.12) and the image is built and run in CI, but the server, domain and registry account are placeholders marked `TODO` until they exist (ROADMAP M9). Until then, the API runs on a dev machine behind a quick tunnel.

## One-time setup

1. **A VPS** with a recent Ubuntu or Debian, amd64, reachable over SSH with your key as `root` (Kamal installs Docker on first setup). 2 GB of memory is plenty. Allow only ports 22, 80 and 443; ideally 80 and 443 only from [Cloudflare's addresses](https://www.cloudflare.com/ips/).
2. **The API's domain on Cloudflare:** an `A` record for the API's host name (e.g. `api.<domain>`) pointing at the VPS, **proxied** (orange cloud). SSL/TLS mode **Full (strict)**.
3. **A Cloudflare origin certificate** for that host (SSL/TLS → Origin Server → Create certificate). Save the certificate and key outside the repository, e.g. `~/.config/issho/origin-cert.pem` and `origin-key.pem`.
4. **A registry token:** a GitHub personal access token (classic) with `write:packages`, exported as `KAMAL_REGISTRY_PASSWORD` when deploying. The image is pushed to `ghcr.io/<owner>/issho-backend`; keep the package private.
5. **A production Supabase project**, configured as in [supabase-hosted.md](supabase-hosted.md), and its values in a gitignored `.env.issho-prod` (same names as `.env.issho-dev`). `DATABASE_URL` must be a **session-mode** connection (direct, or the session pooler): the API listens for chat messages and the worker for River jobs.
6. **Fill in `config/deploy.yml`:** every line marked `TODO` (server address, API host, registry account, `SUPABASE_URL`, `ALLOWED_ORIGINS`).
7. **Secrets:** `cp .kamal/secrets.example .kamal/secrets` (gitignored). It reads each value from `.env.issho-prod` and the certificate files, so the file itself holds no secrets. See [secrets.md](secrets.md).
8. **Install Kamal:** `gem install kamal`, or run it from its image: `alias kamal='docker run -it --rm -v "$PWD:/workdir" -v "$SSH_AUTH_SOCK:/ssh-agent" -e SSH_AUTH_SOCK=/ssh-agent -v /var/run/docker.sock:/var/run/docker.sock ghcr.io/basecamp/kamal:v2.12.0'`.
9. **First deploy:** `kamal setup` (installs Docker, starts kamal-proxy, then deploys as below).

## Deploying

From a clean checkout of the commit to ship (Kamal builds from a fresh clone of `HEAD`, so uncommitted changes are never shipped):

```sh
kamal build push        # build the amd64 image (cross-compiled, fast on a Mac) and push it
kamal migrate           # apply new migrations with that image (alias for `app exec --primary "tool migrate up"`)
kamal deploy --skip-push
```

Migrations run **before** the new code, so the old code must work with the new schema for the minute in between; keep migrations additive, as they already are. kamal-proxy only switches traffic once the new API container's `GET /up` answers 200, which needs the database, so a broken `DATABASE_URL` never replaces a working container.

Then check:

```sh
curl https://<api host>/up                                     # ok
curl -X POST -H 'Content-Type: application/json' -d '{}' https://<api host>/issho.v1.HealthService/Check   # the commit's version
kamal logs-api          # or logs-worker
```

**Seeding and Google tools** run the same way, inside the image: `kamal app exec --primary "tool seed /data/citypacks/tokyo.json"` (the catalogue is seeded too).

**Rolling back:** `kamal rollback <version>` (versions are commit hashes; `kamal app containers` lists them). Migrations aren't rolled back.

## To check on the first real deploy

- Group chat's WebSocket (`/ws/chat`) through Cloudflare and kamal-proxy: open a group chat in the app and send a message from another account.
- The API sees Cloudflare's address, not the client's (nothing uses client addresses yet; if something does, read `CF-Connecting-IP`).
- Point the app's and the sites' API URL at the new host: `EXPO_PUBLIC_API_URL`, and `scripts/deploy-web.sh https://<api host>`.
