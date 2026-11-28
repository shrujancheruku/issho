# .kamal

Kamal's secrets file for dev, `secrets.dev`. Kamal evaluates it on every call and gives the result to the containers and to its own SSH, registry and TLS settings.

**It holds no value.** Each line names a secret in Google Secret Manager (project `issho-dev-512213`) and reads it with `kamal secrets fetch --adapter gcp_secret_manager`, which runs `gcloud secrets versions access`. CI is signed in to Google Cloud by Workload Identity Federation as a service account that can only access secrets, so the file works only there ([docs/deploy.md](../docs/deploy.md)). `scripts/deploy-config.test.sh` fails if a line is anything but a comment, a fetch or an extract.

| Variable | Secret | Used for |
|---|---|---|
| `DATABASE_URL` | `dev-database-url` | The API's database connection (the transaction pooler) |
| `SUPABASE_AUTH_URL` | `dev-supabase-auth-url` | The API's Auth address, for checking sign-in tokens (public; kept here so the project's ref stays out of the repository) |
| `WORKER_DATABASE_URL` | `dev-worker-database-url` | The worker's database connection (a session pooler) |
| `RESEND_API_KEY`, `EMAIL_FROM` | `dev-resend-api-key`, `dev-email-from` | The worker's email |
| `SSH_PRIVATE_KEY` | `dev-ssh-private-key` | Kamal's login to the VM as `kamal` |
| `ORIGIN_CERT`, `ORIGIN_KEY` | `dev-origin-cert`, `dev-origin-key` | TLS between Cloudflare and kamal-proxy |
| `EXPO_PUBLIC_SUPABASE_URL`, `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | `dev-supabase-url`, `dev-supabase-publishable-key` | The web build's sign-in values, given to `apps/web/Dockerfile` as builder secrets (`config/deploy.web.yml`) |
| `KAMAL_REGISTRY_PASSWORD` | none | The workflow's own `GITHUB_TOKEN`, in its environment |

`dev-ssh-host-key` (the VM's public host key, read by `scripts/dev-ssh-pin.sh` before Kamal connects, see [docs/deploy.md](../docs/deploy.md#where-the-secrets-are-and-how-ci-gets-at-them)), `dev-migrate-database-url` and `dev-cloudflare-api-token` are read by `scripts/deploy-dev.sh` (the migration and the admin's deploy), not by Kamal.

`.gitignore` ignores `.kamal/secrets*` except this file. Another destination's file (production's, later) is its milestone's to add the same way.
