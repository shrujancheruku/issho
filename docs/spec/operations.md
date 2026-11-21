# Technical operations

From [Technical operations](https://github.com/shrujancheruku/issho/issues/573), on the stack in [Architecture and stack](architecture.md). It carries out the two promises in [Running Issho](running-issho.md#promises-that-bind-the-stack): no outing fails because of a fault in Issho, and at most a day of data is lost.

## Where it runs

| Part | Where |
|---|---|
| `api`, `worker` and the Expo server | Three containers on one Compute Engine VM in Singapore (`asia-southeast1`), `e2-small` to start, deployed with Kamal 2 |
| The database and sign-in | A new production Supabase project in Singapore, on the Pro plan |
| The admin | A static Cloudflare Worker at `admin.joinissho.com` |
| DNS and the proxy | Cloudflare, in front of everything. It caches the public pages. |
| Secrets | Google Secret Manager, read by Kamal at deploy |

- Deploys run from CI. Nothing that deploys is installed on the server.
- CI signs in to Google Cloud by Workload Identity Federation, with no stored keys.
- Cloudflare is a vendor with a case made under [the rule for outside services](architecture.md#the-rule-for-outside-services): it already holds the DNS and costs nothing, and Google's static hosting means Firebase or a paid load balancer.
- The VM is revisited at the soft launch, or the first time the VM itself causes an incident. The image is the same on Cloud Run, so the move is small.
- **Check in the build's first deploy ticket:** Kamal reads secrets from Secret Manager. If not, CI reads them and passes them to Kamal.

## Environments

Three, with no staging.

| Environment | What | Deployed |
|---|---|---|
| Local | The Supabase CLI stack with fixtures | By hand |
| Dev | The `issho-dev` Supabase project and a second Kamal destination on its own `e2-micro` in the same region | On every merge to main |
| Production | The above | Only by a release |

- Dev is used by preview builds and the paid readers. It never holds real people.
- Dev has its own VM so a dev fault can never take memory from production. It can be stopped by hand in quiet weeks.

## What the promise means

"No outing fails because of a fault in Issho" is three rules, not an uptime figure.

- **An outage on the day fails no outing.** Everything a person needs on the day (the time, the meeting point, the group's first names) is already in the reminder emails.
- **Jobs survive outages.** Jobs live in Postgres, so a restart loses none. Every job that touches an outing retries until it succeeds and raises an alert when it can't.
- **No deploy in the 2 hours before a set outing's start**, unless it fixes an urgent alert.

## Backups and the restore

- **Supabase's daily backups**, kept 7 days. No point-in-time recovery before the soft launch.
- **Our own nightly dump** to an encrypted Cloud Storage bucket, kept 30 days, so one vendor's failure doesn't take both copies.
- **A restore test** by script, monthly and before each stage: the latest dump is restored into a scratch database, row counts are compared, and the result is a line in the digest.
- A missed backup or a failed restore test is an urgent alert.
- **The deletion ledger.** Each account deletion is recorded as the account's ID and a time, and nothing else. After any restore the ledger is replayed before the database takes traffic, so a restore never brings a deleted account back. The ledger is also the proof that a deletion happened.
- The privacy policy says backups keep deleted data for up to 30 days ([Privacy](privacy.md#deleting-an-account)).

## Errors and logs

- **Sentry** events carry the account's ID and nothing else about a person: no email, no IP address, no request bodies or headers, no session replay and no performance tracing. Events are kept 30 days.
- **Logs.** The API, the Expo server and the proxy log the account's ID and the route, and no IP address or email. Logs go to Cloud Logging and are kept 30 days.
- Issho keeps no IP address anywhere ([Privacy](privacy.md#no-phone-number-and-the-sign-in-log)).

## Technical alerts

The urgent alerts in [Running Issho](running-issho.md#alerts) arrive through the Issho app, which can't report its own outage. Technical alerts go through Google Cloud Monitoring, to email and the Google Cloud phone app.

- **Urgent:**
  - the API or the web is down for 5 minutes;
  - the worker's heartbeat is missing for 15 minutes;
  - a job touching an outing in the next 24 hours has failed its retries;
  - a backup was missed or a restore test failed;
  - push or email delivery is failing;
  - the retention job finds a record past its lifetime.
- **Everything else** (other failed jobs, new Sentry errors) is a line in the 08:00 digest.

## Cost

- One Google Cloud budget of US$150 a month, alerting at 50%, 90% and 100%.
- A daily quota cap on each paid API (Places, Routes, Vertex, Translation), so a bug stops instead of billing.
- Supabase's spend cap is on. Sentry, Resend and Expo are on free or fixed plans.

## Releases

- One version number for everything, kept by release-please. The owner merging the release PR is the release.
- **Server side:** the release deploys the API, the worker, the web and the admin to production, with migrations first.
- **The app:** the release publishes an over-the-air update. A store build is submitted through EAS only when native code changed.
- **Compatibility:** migrations and the API stay compatible with the previous app release, checked by `buf breaking` in CI.
- **The minimum-version gate:** a build older than the minimum shows "update Issho" and nothing else.

## The store reviewers' way in

- One reviewer account, signed in by an email and a password. The app shows the password field only for that address.
- The account lives in a **demo city** ([Cities](cities.md#a-demo-city)), which holds seeded plans and scripted members, so a reviewer sees an outing at each stage and a chat room and never meets a real person.
- **Check in the build's first sign-in ticket:** Supabase Auth can take a password for one account while every other account uses an email code.

## The second sign-in step

- The step is required by where a call is made, not by role. Every admin RPC requires it. The Making tab's RPCs in the member app don't, since they reach no personal data.
- Enrolling a TOTP factor is forced at the first admin sign-in. An admin session asks for the step again after 12 hours.
- **A lost device:** the owner removes that person's factor and they enrol again. The owner enrols two factors and falls back on the Supabase dashboard.

## The sign-in log

- The log is Supabase Auth's own audit log, written to the database: each sign-in and sign-out with its time and the account. Issho has no table or capture code of its own for it.
- It holds no IP address. Supabase records none for sign-ins and sign-outs.
- The retention job deletes its rows at 12 months. Nothing in the admin reads it; the owner produces it only for a lawful request, and that is written in the staff log.
- Whether a log with no IP address is enough is a question for the lawyer.
- **Check in the build's first sign-in ticket:** what Supabase does with a deleted account's rows.

## Retention and deletion

- **One registry.** The `retention` package holds one entry per record type: its lifetime from [Privacy](privacy.md#what-is-kept-and-for-how-long) and its delete query.
- **One nightly job** runs every entry and records what it removed, as counts only.
- **CI fails** when a table holding personal data has no entry.
- **The proof** is one line in the digest, "nothing past its lifetime". Anything found past its lifetime is an urgent alert.
- **Deleting an account** also removes the Supabase Auth user and the push tokens, and writes the deletion ledger.
