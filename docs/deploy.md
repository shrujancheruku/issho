# Deploying dev

How dev is deployed, how to roll it back and how to stop its VM in a quiet week. Production, releases, backups, Sentry and alerts are later milestones' ([Technical operations](spec/operations.md)); this is dev only.

Dev is deployed from CI on every merge to `main`, by Kamal 2, onto its own VM. Nothing that deploys is installed on the VM, and nothing deploys from a laptop.

## What dev is

| Part | Where | Address |
|---|---|---|
| The API (`api`) and the worker (`worker`) | Two containers from one image, `ghcr.io/shrujancheruku/issho-server`, on the dev VM | `api-dev.joinissho.com` |
| The web, the member app server-rendered (`apps/web`) | One container from `ghcr.io/shrujancheruku/issho-web` on the same VM | `dev.joinissho.com` |
| The admin (`apps/admin`) | A Cloudflare Worker, `issho-admin-dev`, static files and a small forwarder | `admin-dev.joinissho.com` |
| The database and sign-in | The `issho-dev` Supabase project | |
| The VM | `issho-dev`, an `e2-micro` (1 GB of memory) in `asia-southeast1-b`, Google Cloud project `issho-dev-512213`, on the static address `issho-dev-ip` | |

Cloudflare is in front of all of it (proxied, SSL mode Full (strict)). The VM takes ports 80 and 443 only from Cloudflare, and answers with a Cloudflare origin certificate, which kamal-proxy holds. One kamal-proxy on the VM routes by name: each name has one service.

## A deploy

`.github/workflows/deploy-dev.yml` runs on a push to `main`. One deploy at a time. The job signs in, finds the VM and pins its SSH host key, runs `make deploy-dev` ([`scripts/deploy-dev.sh`](../scripts/deploy-dev.sh)), then `make deploy-dev-verify`:

1. **Docker on the VM.** `kamal server bootstrap` installs it on the first run and does nothing after.
2. **Build and push both images** to ghcr.io, tagged with the commit. A build that fails changes nothing.
3. **Migrate `issho-dev`** with goose (`backend/cmd/migrate up`), run on the CI runner against the migration role's connection. **This is before any container is replaced**: the script stops at the first failing step (`set -e`), so a failed or half-applied migration leaves the old containers serving and nothing after it runs. `scripts/deploy-dev.test.sh` proves the order with stand-ins for `kamal`, `go`, `pnpm` and `gcloud`; a failed migration, a failed build and a failed API deploy each leave the later steps uncalled.
4. **Deploy the API and the worker** (`kamal deploy --skip-push`: the VM pulls the images pushed in step 2, so what runs is what was built). kamal-proxy sends the new API container traffic only when `GET /up` answers 200, then drains the old one.
5. **Deploy the Expo server** the same way, `GET /up` of `apps/web/server.js` being its probe.
6. **Deploy the admin**: `pnpm --filter @issho/admin build`, then `wrangler deploy --env dev`.
7. **Check from outside** (`scripts/deploy-dev-verify.sh`): the API, the web's address and the admin's Worker each report the commit just deployed through the API's public health check; the web's `/plans` holds its words in the HTML a client with no JavaScript gets; and a real headless Chrome shows `API version <commit>` on the web's You tab and on the admin's page. A deploy that left an old API running fails here.
8. The VM's memory is printed (`free -m` and `docker stats`).

### Where the secrets are and how CI gets at them

- **No cloud key or token is stored in GitHub.** The workflow signs in to Google Cloud by Workload Identity Federation: GitHub gives the run a short-lived OIDC token (`id-token: write`), and Google exchanges it for the access of `issho-deploy@issho-dev-512213.iam.gserviceaccount.com` through the provider `projects/94737271594/locations/global/workloadIdentityPools/github/providers/issho`, which accepts only this repository. That account can only access secrets in Secret Manager. The only secret the job holds is its own `GITHUB_TOKEN` (`packages: write`), which pushes the images, is what the VM pulls with, and expires with the job.
- **Kamal reads its secrets from Secret Manager itself.** This answers the spec's check ([Technical operations](spec/operations.md#where-it-runs)). Kamal 2.12's `kamal secrets fetch --adapter gcp_secret_manager` runs `gcloud secrets versions access`, which the service account's role allows, and `.kamal/secrets.dev` uses it: the file names secrets and holds no value. CI does not have to read them and hand them over. The two secrets Kamal does not read (the migration's connection and the Cloudflare token) are read by `scripts/deploy-dev.sh` with the same `gcloud` call, put in the environment of the one command that needs each, and masked in the log. Kamal's `config` command prints the SSH key it read, so never run it where its output is kept.
- The secrets are `dev-worker-database-url`, `dev-resend-api-key`, `dev-email-from`, `dev-ssh-private-key`, `dev-origin-cert`, `dev-origin-key`, `dev-ssh-host-key`, `dev-migrate-database-url` and `dev-cloudflare-api-token`. The Cloudflare token's stored value ends in a newline, which Cloudflare rejects; the script strips it. The API reads `dev-database-url` (the transaction pooler's URL, as `DATABASE_URL`) and `dev-supabase-auth-url` (`https://<ref>.supabase.co/auth/v1`, as `SUPABASE_AUTH_URL`): `cmd/api` uses them to keep accounts and check sign-in tokens. The web image is built with the sign-in's two values, `dev-supabase-url` (`https://<ref>.supabase.co`) and `dev-supabase-publishable-key`, which Kamal reads by name and gives the build as builder secrets (`config/deploy.web.yml`; `apps/web/Dockerfile` mounts them for the one `expo export`, so neither stays in the image). **Founder step:** create both in Secret Manager before the deploy that carries #639; without them the preflight stops the deploy, naming the missing one. The native builds read the same values from EAS environment variables, which the founder sets ([docs/secrets.md](secrets.md#dev-google-secret-manager)). The Auth URL is public; it is in Secret Manager only so the project's ref stays out of this repository. Both must exist before a deploy: the script's first step checks every secret `.kamal/secrets.dev` names, by reading it with `gcloud secrets versions access` (the permission the account already has; the value is discarded, never printed), and stops before Kamal runs if one is missing.
- **The VM's address is stored nowhere.** The service account has a custom role, `isshoDeployAddressReader`, with only `compute.addresses.get`, and `scripts/dev-host.sh` asks Google Cloud for the reserved address `issho-dev-ip` each run. The workflow masks it (`::add-mask::`) the moment it is read, before Kamal prints the host it connects to: this repository's Actions logs are public.
- **The VM's SSH host key is pinned.** Its public key is the secret `dev-ssh-host-key` (one line, `ssh-ed25519 AAAA…`, no address). Before Kamal connects, `scripts/dev-ssh-pin.sh` writes a `known_hosts` line from the address just looked up and that key, and an ssh config with `StrictHostKeyChecking yes` that reads only that file; both destinations give it to Kamal (`ssh: config:`, from `DEV_SSH_CONFIG`). An unknown or changed key is refused, never added, so a machine posing as the VM gets no secret. If the VM is rebuilt, its new key goes into `dev-ssh-host-key` (`gcloud compute instances get-guest-attributes issho-dev --zone asia-southeast1-b --query-path=hostkeys/`, the `ssh-ed25519` one) before the next deploy, or the deploy fails at its first Kamal step.

## How the browser reaches the API

The API has no CORS, by design: no browser origin is trusted to call it from another. Each web page therefore calls it on **its own origin**.

- **The web**, at `dev.joinissho.com`: the Expo server forwards the RPC paths (`/issho.v1.*`, nothing else) to `https://api-dev.joinissho.com` (`apps/web/api-proxy.js`) and serves every other path as the page. The app is built with `EXPO_PUBLIC_API_URL=https://dev.joinissho.com`. Sharing the name by path in kamal-proxy was tried first and refused: it wants TLS settings on the root path's service.
- **The admin**, at `admin-dev.joinissho.com`: the Worker forwards the same paths to `https://api-dev.joinissho.com` and serves everything else as static files (`apps/admin/README.md`).
- **Phones** are not a browser: they call `api-dev.joinissho.com` directly.

Both forwarders drop the headers that carry the member's address (`cf-connecting-ip`, `x-forwarded-for`, `x-real-ip`, `forwarded`, `true-client-ip` and the like), since Issho keeps no IP address anywhere ([ADR 0005](adr/0005-no-phone-number-and-no-ip-address.md)); the API sees the forwarder's address instead. The forwarders drop them only after kamal-proxy, which sits before them, has logged each request, so the zone setting below is what keeps the address out of the proxy's log.

### A required Cloudflare zone setting

**Turn on the managed transform "Remove visitor IP headers"** on the `joinissho.com` zone (Cloudflare dashboard, Rules, Settings, Managed Transforms). It is a zone setting made by hand; nothing in this repository sets it, and the deploy does not check it.

- **Why:** Cloudflare puts the visitor's address in `X-Forwarded-For` (and `CF-Connecting-IP`) on every request to the VM. kamal-proxy's request log writes `remote_addr` from that header on each request, and it logs before either forwarder can drop anything. Phones call `api-dev.joinissho.com` directly, so their requests reach it with Cloudflare's headers as they are. Without the transform the proxy's log holds every member's address, which [ADR 0005](adr/0005-no-phone-number-and-no-ip-address.md) and [Technical operations](spec/operations.md#logs) rule out. With it, the origin sees only Cloudflare's edge address.
- **To confirm it,** read one request from the proxy's log in a CI run or on the VM (`kamal proxy logs -d dev`, never on a laptop) and check its `remote_addr`: it must be one of Cloudflare's published ranges ([cloudflare.com/ips](https://www.cloudflare.com/ips/)), not the address of the machine that made the request. Make the request from a machine whose address you know. Check again after any change to the zone's rules, and before the first production deploy.

Why a forwarder and not CORS, what it costs and what would reverse it: [ADR 0006](adr/0006-browsers-reach-the-api-on-their-own-origin.md).

## Checking the API after a deploy

A request with no sign-in token must be refused. It shows the deployed API is up and holds its Auth address:

```sh
curl -s -X POST https://api-dev.joinissho.com/issho.v1.AccountService/GetAccount -H 'content-type: application/json' -d '{}'
```

Expect a body with code `unauthenticated` (HTTP 401). The owner runs this once after the first merge that carries the database and Auth settings.

## Rolling back

A rollback is a deploy of an earlier commit.

1. **Preferred:** revert the bad change in a PR and merge it. The merge deploys the revert.
2. **Quicker, no PR:** in GitHub, Actions, `deploy-dev`, Run workflow, and give the last good commit's SHA as `ref`. The run builds that commit and deploys it, then checks that the API reports it.

A rollback never undoes a migration: goose's `down` is a manual act on the database, and nothing here runs it. A migration is written compatible with the release before it ([Technical operations](spec/operations.md#releases)), so the older code runs on the newer schema. If a migration itself is the fault, fix it forward.

## Stopping the dev VM in a quiet week

Dev has its own VM so it can be stopped by hand ([Technical operations](spec/operations.md#environments)). Stopping it stops the API, the worker and the web, and the admin's page loads but cannot reach the API.

```sh
gcloud compute instances stop issho-dev --zone asia-southeast1-b --project issho-dev-512213
gcloud compute instances start issho-dev --zone asia-southeast1-b --project issho-dev-512213
```

- The containers come back on their own when Docker starts: Kamal runs them with `--restart unless-stopped`, and kamal-proxy as well. Confirm with `make deploy-dev-verify` (it only reads public addresses) once the VM has been up a minute.
- The static address keeps its name while the VM is stopped, and Google bills a reserved address that nothing uses at a higher rate than one in use: a few days is fine, months is not.
- **Start it before merging to `main`.** A deploy to a stopped VM fails at its first step (it can't reach the VM), changing nothing.
- Cloudflare shows its own error page for the dev names while the VM is stopped.

## When something is wrong

| What you see | Where to look |
|---|---|
| The run fails at "Find the VM" | The service account lost `isshoDeployAddressReader`, or `issho-dev-ip` is gone or has no address (`scripts/dev-host.sh`) |
| It fails signing in to Google | The provider or the service account changed; `projects/94737271594/.../providers/issho` must still accept this repository |
| It fails at the migration | The run's log, and nothing was replaced. Fix the migration, push |
| It fails at the first Kamal step with a host key error | The VM's host key is not the one in `dev-ssh-host-key`. If the VM was rebuilt, store its new key (above); if not, something else answered at its address: find out what before deploying |
| It fails at a Kamal step | The run's log names the host and the command. Kamal holds a lock on the VM during a deploy; a run killed halfway can leave it (`kamal lock release -d dev`, from a CI run, not a laptop) |
| The check at the end fails | The part that failed says which address showed which version. The API may be up with the old version: look at the container logs with `kamal app logs -d dev` in a run, or on the VM |
| The VM is out of memory | The deploy prints `free -m` and `docker stats`. Three containers and the proxy on 1 GB are tight; move the web or the API to the larger machine type (`e2-small`, as in production) before they fail |

## Not here yet

Production's destination, release-driven deploys, backups, Sentry, alerts and the Google Cloud logging of the containers (all later milestones'), and a deploy hold for the two hours before an outing ([Technical operations](spec/operations.md#what-the-promise-means)), which has no outings to protect until they exist.

## Seeding the dev city

A freshly migrated `issho-dev` has no city, so the API refuses every city and nobody can sign in. The founder, not CI and not a worker, seeds it. The city row is upserted to the fixture's values (state, time zone, currency, names), and the plan rows are rewritten (deleted, then inserted) with the fixture's content, so running it again is safe but puts the fixture back over any change made to those rows by hand.

First, once, mark the dev database as dev. The marker is a database setting stored in the database itself (`pg_db_role_setting`), so the operator can't supply it the way they supply a ref, and a connection can't fake it: the seed reads the stored setting, not the session's, so `?options=-c issho.environment=dev` or `SET` do nothing. Run this against issho-dev only, and never on production:

```sql
alter database postgres set issho.environment = 'dev';
```

Then seed:

```sh
export DATABASE_URL=...            # dev's migration connection (secret dev-migrate-database-url)
export SEED_DEV_PROJECT_REF=...    # the issho-dev project's ref
make seed-dev
```

`cmd/seed` refuses any database that isn't on this machine unless `--hosted-dev` is given. Then it checks three things before writing anything: the URL (and the `PG*` environment) sets no session parameters such as `options`, the target is the project named by `SEED_DEV_PROJECT_REF` (its `db.<ref>.supabase.co` host, or `postgres.<ref>` on the pooler, judged on the connection pgx would make, so a `?user=` in the query counts), and the database's stored `issho.environment` setting is exactly `dev`. The marker check and the writes share one transaction, so the database that was checked is the one written. The ref is what the operator typed, so on its own it would let production's URL through with production's ref; the marker is why that fails, since production never has it. Take the ref from the dev project anyway. The marker is set by hand, in no committed file, and the local `make seed` never reads it.

Check afterwards with the public `ListPlans` call, which needs no token and lists the fixture's plans once the city exists:

```sh
curl -s -X POST https://api-dev.joinissho.com/issho.v1.PlanService/ListPlans -H 'content-type: application/json' -d '{"city":"da-nang-hoi-an"}'
```
