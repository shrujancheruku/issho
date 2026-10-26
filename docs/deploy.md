# Deploying the backend

The Go API and the River worker run on one VPS, deployed with **Kamal 2** from a single image (`backend/Dockerfile`). Cloudflare sits in front of the API; Supabase (database and auth) stays hosted. The admin SPA and the web build are separate static sites on Cloudflare Workers ([../apps/admin/README.md](../apps/admin/README.md)).

```
app / admin SPA ──HTTPS──▶ Cloudflare ──HTTPS (origin certificate)──▶ kamal-proxy ──▶ api container ─┐
                                                                                      worker container ─┴──▶ Supabase Postgres
```

**Status:** live since 2026-10-26 at `https://api.joinissho.com` (first `kamal setup` from a dev machine), against the `issho-dev` Supabase project. Deploys are automatic on merge (see *Automatic deploys*); the backup accessory is off until its R2 bucket exists. The origin certificate (15 years, `api.joinissho.com` only) is in `~/.config/issho/` on the deploying machine. Mind that a PEM pasted from the dashboard has no blank lines inside it, or Kamal's proxy can't read the key.

## Domain: joinissho.com

Registered at Porkbun (2026-10-26); its DNS is moved to Cloudflare, which also serves the sites.

| Host | What | Where |
|---|---|---|
| `joinissho.com`, `www.joinissho.com` | marketing page (`site/`) | Worker `issho-site` |
| `app.joinissho.com` | the app's web build | Worker `issho-web` |
| `admin.joinissho.com` | admin SPA | Worker `issho-admin` |
| `api.joinissho.com` | Go API | the VPS, through Cloudflare (proxied) |

The Workers' `wrangler.jsonc` files claim their host names as custom domains, so a deploy creates their DNS records. The `workers.dev` addresses keep working.

**Moving DNS to Cloudflare (owner, once):** in Cloudflare, *Add a domain* → `joinissho.com`, Free plan; delete the imported Porkbun parking records; at Porkbun, *Domain → Authoritative nameservers* → Cloudflare's two (and switch off Porkbun's DNSSEC first, if on). The zone is active once Cloudflare sees the change (usually under an hour). Then SSL/TLS mode **Full (strict)**.

## One-time setup

1. **The server** (done 2026-10-26): Compute Engine VM `issho-api` in project `issho-dev-512213`, `asia-northeast1-b` (Tokyo, near the database in Seoul), `e2-small` (2 GB), Ubuntu 24.04, 20 GB disk, static address `35.243.76.67` (reserved as `issho-api`). Its startup script installs Docker, adds a 2 GB swap file and unattended upgrades, and puts the `kamal` user (created from the SSH key in the instance metadata) in the `docker` group: Compute Engine has no root login, so Kamal connects as `kamal` (`ssh.user`). Firewall `issho-api-web-from-cloudflare` lets 80 and 443 in only from [Cloudflare's addresses](https://www.cloudflare.com/ips/) (tag `issho-api`); SSH is the project's default rule.
2. **The API's domain on Cloudflare:** an `A` record for `api.joinissho.com` pointing at the VPS, **proxied** (orange cloud). SSL/TLS mode **Full (strict)**.
3. **A Cloudflare origin certificate** for that host (SSL/TLS → Origin Server → Create certificate). Save the certificate and key outside the repository, e.g. `~/.config/issho/origin-cert.pem` and `origin-key.pem`.
4. **A registry token:** a GitHub personal access token (classic) with `write:packages`, exported as `KAMAL_REGISTRY_PASSWORD` when deploying. The image is pushed to `ghcr.io/<owner>/issho-backend`; keep the package private.
5. **A production Supabase project**, configured as in [supabase-hosted.md](supabase-hosted.md), and its values in a gitignored `.env.issho-prod` (same names as `.env.issho-dev`). `DATABASE_URL` must be a **session-mode** connection (direct, or the session pooler): the API listens for chat messages and the worker for River jobs.
6. **Fill in `config/deploy.yml`:** every line marked `TODO` (server address, API host, registry account, `SUPABASE_URL`, `ALLOWED_ORIGINS`).
7. **Secrets:** `cp .kamal/secrets.example .kamal/secrets` (gitignored). It reads each value from `.env.issho-prod` and the certificate files, so the file itself holds no secrets. See [secrets.md](secrets.md).
8. **Install Kamal** (Kamal 2 needs Ruby 3.2+; macOS ships 2.6, so the image is simpler; add `-v ~/.config/issho:/root/.config/issho:ro` so the secrets file finds the certificate, and on OrbStack or Docker Desktop pass the SSH agent as `-v /run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock -e SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock`): `gem install kamal`, or run it from its image: `alias kamal='docker run -it --rm -v "$PWD:/workdir" -v "$SSH_AUTH_SOCK:/ssh-agent" -e SSH_AUTH_SOCK=/ssh-agent -v /var/run/docker.sock:/var/run/docker.sock ghcr.io/basecamp/kamal:v2.12.0'`.
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

## Automatic deploys

`.github/workflows/deploy-api.yml` deploys every change to `backend/`, `data/`, `config/deploy.yml` or `config/server/` on main (or on demand), without production secrets in GitHub:

1. The job builds and pushes the image with Kamal, using its own short-lived `GITHUB_TOKEN` (the package is linked to the repository by the image's `org.opencontainers.image.source` label).
2. It connects as `deploy@<server>` with the `DEPLOY_SSH_KEY` secret (host key pinned by the `DEPLOY_HOST_KEY` variable) and writes `<commit> <token>` to it. That key's `authorized_keys` line is `restrict,command="sudo -n -u kamal /opt/issho/deploy.sh"`, and `/etc/sudoers.d/issho-deploy` allows exactly that, so the key can do nothing else.
3. [`config/server/deploy.sh`](../config/server/deploy.sh) (installed root-owned at `/opt/issho/deploy.sh`) refuses a commit that isn't on `origin/main`, checks it out in `/opt/issho/repo`, and runs Kamal from its image against this server (`kamal` connects to itself with `~kamal/.ssh/id_self`, registered in the instance's SSH-key metadata): `registry login`, `migrate`, `deploy --skip-push`, then logs out of the registry.
4. The job checks that `HealthService.Check` reports the new commit.

**Changing a production secret:** edit `.env.issho-prod` (or the certificate), then copy it to the server, which deploys with its own copy: `grep -v '^CLOUDFLARE_DNS_TOKEN=' .env.issho-prod | ssh kamal@<server> 'cat > /opt/issho/env'` (and `.kamal/secrets` to `/opt/issho/kamal-secrets`, the certificate files to `/opt/issho/certs/`), then rerun the workflow. **Changing `deploy.sh`:** it runs from `/opt/issho/deploy.sh`, not the checkout, so reinstall it after merging: `scp config/server/deploy.sh kamal@<server>:/tmp/ && ssh kamal@<server> 'sudo install -o root -m 755 /tmp/deploy.sh /opt/issho/ && rm /tmp/deploy.sh'`.

**Manual deploys** from a dev machine still work as above; Kamal then leaves your registry login on the server, so log it out afterwards (`ssh kamal@<server> docker logout ghcr.io`).

## Off-site backups

A Kamal accessory, `backup` (`config/deploy.yml`), runs `config/backup/backup.sh` in the `postgres:17-alpine` image (the database's major version). Every night at `BACKUP_AT` (UTC hour, default 3) it `pg_dump`s the `app`, `goose`, `river` and `auth` schemas in the custom format and uploads the file to a Cloudflare R2 bucket, signed with `curl --aws-sigv4`. Live sessions and one-time tokens are left out, so a leaked dump can't open anyone's account. A failed night is logged and retried the next night.

**One-time setup** (needs the Cloudflare account):

1. Create an R2 bucket (e.g. `issho-backups`) and an **R2 API token** with *Object Read & Write* on that bucket only. Put its key pair in `.env.issho-prod` as `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` (`.kamal/secrets` reads them), and the account id and bucket in `config/deploy.yml` (`TODO`s).
2. In the bucket's settings, add a **lifecycle rule** deleting objects after 30 days: that's the retention.
3. `kamal accessory boot backup`, then check the setup once: `kamal accessory exec backup "env BACKUP_ONCE=1 sh /backup/backup.sh"`, which should print `backup: uploaded issho-….dump`. `kamal accessory logs backup` shows each night.

**Restoring** (not yet rehearsed against a hosted project; do a drill into a scratch Supabase project before real users): download a dump from R2, then into an empty project with the same migrations applied:

```sh
pg_restore --list issho-….dump                       # what it holds
pg_restore --no-owner --no-privileges --data-only --schema=auth --table=users --table=identities -d "$NEW_DATABASE_URL" issho-….dump
pg_restore --no-owner --no-privileges --data-only --schema=app -d "$NEW_DATABASE_URL" issho-….dump
```

Supabase creates `auth` itself, so only its data comes back (people's accounts; they sign in again). `goose` and `river` hold bookkeeping that the new project's own migrations recreate.

## To check on the first real deploy

- Group chat's WebSocket (`/ws/chat`) through Cloudflare and kamal-proxy: open a group chat in the app and send a message from another account.
- The API sees Cloudflare's address, not the client's (nothing uses client addresses yet; if something does, read `CF-Connecting-IP`).
- Point the app's and the sites' API URL at the new host: `EXPO_PUBLIC_API_URL`, and `scripts/deploy-web.sh https://<api host>`.
