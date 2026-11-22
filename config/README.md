# config

Kamal 2's configuration for dev ([docs/deploy.md](../docs/deploy.md) says how a deploy runs). Kamal runs only in CI, from `.github/workflows/deploy-dev.yml`, at the release in `kamal-version`; nothing here is run from a laptop.

| File | Holds |
|---|---|
| `deploy.yml` | The backend service `issho`: the API (behind kamal-proxy) and the worker (no proxy), both from `backend/Dockerfile`. What every environment shares |
| `deploy.dev.yml` | The backend's dev destination: the host, the pinned SSH host key and the API's name |
| `deploy.web.yml` | The Expo server, a service of its own (`issho-web`, from `apps/web/Dockerfile`) on the same host and proxy. Used with `-c config/deploy.web.yml` |
| `deploy.web.dev.yml` | The Expo server's dev destination: its host, the pinned SSH host key, its name and the address its build calls |
| `kamal-version` | The Kamal release CI installs and the config test runs |

Every Kamal call passes `-d dev`; `require_destination` makes it refuse one without. The secrets are named in `.kamal/secrets.dev` ([`.kamal/README.md`](../.kamal/README.md)). `DEPLOY_HOST`, the VM's address, comes from the environment, so it is not in the repository; so does `DEV_SSH_CONFIG`, the ssh config `scripts/dev-ssh-pin.sh` writes, which refuses any host key but the VM's.

Never run `kamal config` where its output is kept: it prints the SSH key it read.

## Testing

`scripts/deploy-config.test.sh` (`make deploy-test`, part of `make test`) loads both services through Kamal itself, in its Docker image, with a stand-in for `gcloud` that serves made-up secrets. It checks the hosts, that SSH reads the pinned host key, the roles, the worker's secrets, the proxy's names, paths and probe, TLS, that a certificate keeps its lines, that the version is a string, and that a call without a destination is refused. It needs Docker and touches no server.
