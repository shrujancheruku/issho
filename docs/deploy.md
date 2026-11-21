# Deploying dev

How dev is deployed, how to roll it back and how to stop its VM in a quiet week. Production, releases, backups, Sentry and alerts are later milestones' ([Technical operations](spec/operations.md)); this is dev only.

Dev is deployed from CI on every merge to `main`, by Kamal 2, onto its own VM. Nothing that deploys is installed on the VM, and nothing deploys from a laptop.

## What dev is

| Part | Where | Address |
|---|---|---|
| The API (`api`) and the worker (`worker`) | Two containers from one image, `ghcr.io/shrujancheruku/issho-server`, on the dev VM | `api-dev.joinissho.com`, and `dev.joinissho.com/issho.v1.*` |
| The web, the member app server-rendered (`apps/web`) | One container from `ghcr.io/shrujancheruku/issho-web` on the same VM | `dev.joinissho.com` |
| The admin (`apps/admin`) | A Cloudflare Worker, `issho-admin-dev`, static files and a small forwarder | `admin-dev.joinissho.com` |
| The database and sign-in | The `issho-dev` Supabase project | |
| The VM | `issho-dev`, an `e2-micro` (1 GB of memory) in `asia-southeast1-b`, Google Cloud project `issho-dev-512213`, on the static address `issho-dev-ip` | |

Cloudflare is in front of all of it (proxied, SSL mode Full (strict)). The VM takes ports 80 and 443 only from Cloudflare, and answers with a Cloudflare origin certificate, which kamal-proxy holds. One kamal-proxy on the VM routes by name and path.

## A deploy

`.github/workflows/deploy-dev.yml` runs on a push to `main`. (While the Foundation milestone is built it also runs on a push to `integration/foundation`, so the deploy is proven before that merges.) One deploy at a time. The job signs in, finds the VM, runs `make deploy-dev` ([`scripts/deploy-dev.sh`](../scripts/deploy-dev.sh)), then `make deploy-dev-verify`:

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
- The secrets are `dev-worker-database-url`, `dev-resend-api-key`, `dev-email-from`, `dev-ssh-private-key`, `dev-origin-cert`, `dev-origin-key`, `dev-migrate-database-url` and `dev-cloudflare-api-token`. The Cloudflare token's stored value ends in a newline, which Cloudflare rejects; the script strips it. `dev-database-url` is not read yet, since the API has no database connection until a ticket gives it one.
- **The VM's address is stored nowhere.** The service account has a custom role, `isshoDeployAddressReader`, with only `compute.addresses.get`, and `scripts/dev-host.sh` asks Google Cloud for the reserved address `issho-dev-ip` each run. The workflow masks it (`::add-mask::`) the moment it is read, before Kamal prints the host it connects to: this repository's Actions logs are public.

## How the browser reaches the API

The API has no CORS, by design: no browser origin is trusted to call it from another. Each web page therefore calls it on **its own origin**.

- **The web**, at `dev.joinissho.com`: kamal-proxy sends the RPC paths (`/issho.v1.*`, prefix kept) to the API and every other path to the Expo server. The app is built with `EXPO_PUBLIC_API_URL=https://dev.joinissho.com`.
- **The admin**, at `admin-dev.joinissho.com`: the Worker forwards the same paths to `https://api-dev.joinissho.com` and serves everything else as static files (`apps/admin/README.md`).
- **Phones** are not a browser: they call `api-dev.joinissho.com` directly.

Choosing path routing and a forwarder over adding CORS kept the API unchanged and closed. If a page ever has to call the API from another origin, that needs a CORS rule written in the API's own ticket, with an allowed-origin list, not a loosening here.

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
| It fails at a Kamal step | The run's log names the host and the command. Kamal holds a lock on the VM during a deploy; a run killed halfway can leave it (`kamal lock release -d dev`, from a CI run, not a laptop) |
| The check at the end fails | The part that failed says which address showed which version. The API may be up with the old version: look at the container logs with `kamal app logs -d dev` in a run, or on the VM |
| The VM is out of memory | The deploy prints `free -m` and `docker stats`. Three containers and the proxy on 1 GB are tight; move the web or the API to the larger machine type (`e2-small`, as in production) before they fail |

## Not here yet

Production's destination, release-driven deploys, backups, Sentry, alerts and the Google Cloud logging of the containers (all later milestones'), a host-key pin for the SSH connection (Kamal accepts the VM's key the first time it sees it, and the runner is new each run, so today it checks nothing), and a deploy hold for the two hours before an outing ([Technical operations](spec/operations.md#what-the-promise-means)), which has no outings to protect until they exist.
