<p align="center">
  <img src="assets/brand/logo.svg" alt="Issho 一緒" width="380">
</p>

<h3 align="center">Find your people through the things you already love.</h3>
<p align="center">好きなことから、仲間に出会う。</p>

> 🚧 **v2 is being built.** It is written from scratch in this repo, from the spec in [docs/spec/](docs/spec/README.md). v1's code was deleted when the build started; it stays readable at the [`v1-final`](https://github.com/shrujancheruku/issho/tree/v1-final) tag. The plan and its progress are in the [open issues](https://github.com/shrujancheruku/issho/issues).

## Documentation

| Doc | Read it for |
|---|---|
| [docs/spec/](docs/spec/README.md) | What v2 is: one document per product area, and the [build order](docs/spec/build-order.md) |
| [GLOSSARY.md](GLOSSARY.md) | The terms the spec and the code use |
| [docs/adr/](docs/adr/) | Why things are the way they are: the few decisions that are hard to reverse |
| [GitHub Issues](https://github.com/shrujancheruku/issho/issues) | What's planned and in progress. Large efforts are parent issues with sub-issues. |
| [docs/agents/](docs/agents/) | How agents work here: the tracker, its labels and the build's orchestration |
| [docs/secrets.md](docs/secrets.md) | Where secrets live and how leaks are prevented (**public repo**) |
| [SECURITY.md](SECURITY.md) | Reporting vulnerabilities |

| Path | What |
|---|---|
| [`assets/brand/`](assets/brand/README.md) | Logo and mark, plain and in round blue and white versions, as SVG and PNG; brand colours and the story behind the kanji |
| [`proto/`](proto/README.md) | The API's contract (ConnectRPC over protobuf), where every RPC declares who may call it |
| `backend/` | The Go module: commands `cmd/api` and `cmd/worker`, packages under `internal/` (`testdb` gives each test its own schema in a throwaway Postgres) |
| `packages/api-client/` | The generated TypeScript client of the API ([README](packages/api-client/README.md)) |
| `apps/` | The pnpm workspace's apps, `apps/app` and `apps/admin`, each added by its own ticket |
| `scripts/` | The tracker check, the `buf breaking` wrapper and the brand generators |
| [`e2e/`](e2e/README.md) | The Playwright tests, run against the API and the web build by `make e2e` |
| [`.github/workflows/`](.github/workflows/README.md) | CI's required checks, and how a job that skips still passes |
| `tools/` | Go tools pinned for the repo (gitleaks, goose, sqlc, buf and its Go plugins, actionlint) |
| [`CLAUDE.md`](CLAUDE.md), `.claude/skills/` | Instructions for AI coding agents, and the official Supabase agent skills |
| `.githooks/` | Git hooks: secret scan (pre-commit) and commit-message lint (commit-msg). Enable with `make setup`. |

## Quick start

Prerequisites: Go, pnpm and Docker (running; `make test` needs it).

```sh
make setup          # pnpm install (commitlint, the TypeScript generator) + the git hooks
make gen            # code generation: sqlc, and buf for the Go and TypeScript API code
make lint           # gofmt, go vet, buf lint and breaking, the RPC access-rule check, the client's type check
make test           # backend tests against a throwaway Postgres in Docker, and the client's tests
make secrets-scan   # scan every commit for secrets
```

`make test` starts its own Postgres and gives each test its own schema. It fails with Docker stopped; no flag or environment variable skips the database.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) (`commitlint.config.mjs`). Releases follow SemVer through release-please; v2's first is `2.0.0`.
