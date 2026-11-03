#!/usr/bin/env bash
# Decides which CI jobs a change needs. Reads the changed file paths on stdin
# (one per line) and prints `name=value` lines for $GITHUB_OUTPUT:
#   backend, codegen, image, proto, ts   true/false: run that job
#   ts_filter                            "--affected" when turbo may test only
#                                        the packages a change reaches, else ""
# `--all` ignores stdin and turns everything on (pushes to main, dispatches).
#
# Fail safe: a path no rule below knows turns everything on. Skipping a check
# by mistake is worse than running one too many. Each group says why its paths
# belong to it; tests are in scripts/ci-changes.test.sh.
set -euo pipefail

backend=false codegen=false image=false proto=false ts=false
# ts_pkgs: every TypeScript-relevant path was inside apps/ or packages/, where
# turbo's own dependency graph can pick the packages.
ts_pkgs=true
all=false
[[ "${1:-}" == "--all" ]] && all=true

classify() {
  case "$1" in
    # --- Paths no job tests: documentation, tooling config, deploy and
    # hosting config (checked by the deploy workflows, not by CI), the static
    # site and brand assets. docs/generated is NOT here: it is codegen output.
    docs/generated/*) codegen=true ;;
    docs/* | *.md | .claude/* | site/* | assets/* | config/* | .kamal/* | supabase/* \
      | .githooks/* | .gitignore | .worktreeinclude | .gitleaks.toml | .env.example \
      | version.txt | .release-please-manifest.json | release-please-config.json \
      | skills-lock.json | commitlint.config.mjs | .github/pull_request_template.md \
      | .github/ISSUE_TEMPLATE/* | .github/CODEOWNERS | .github/dependabot.yml) ;;

    # --- Go. The backend job (vet, unit tests, migrations, DB integration
    # tests) and the deploy image both build all of backend/ plus data/, which
    # the image copies in and tests read. go.work wires the module.
    # codegen too: its generators (cmd/tool ui-strings, sqlc, buf plugins) are
    # compiled from the whole backend module, so any Go file can break `make gen`.
    backend/* | data/* | go.work | go.work.sum | .dockerignore)
      backend=true image=true codegen=true ;;

    # --- Protobuf. Generates Go and TS code (codegen drift, api-client) and is
    # what `buf breaking` compares; the Go stubs compile in backend, and the
    # TS client is typechecked in typescript. Turbo gets no filter: the
    # generated client's dependents are cheap to test and easy to miss.
    proto/* | buf.gen.yaml | buf.yaml | buf.work.yaml)
      proto=true codegen=true backend=true image=true ts=true ts_pkgs=false ;;

    # --- TypeScript packages. The ui-strings generator reads the mobile app's
    # English strings and human translations, so those also feed codegen.
    apps/mobile/src/i18n/*) ts=true codegen=true ;;
    apps/* | packages/*) ts=true ;;

    # --- Anything else (workflows, lockfile, workspace/turbo/tsconfig/eslint
    # roots, Makefile, scripts, unknown paths): everything on, and turbo gets
    # no filter.
    *) backend=true codegen=true image=true proto=true ts=true ts_pkgs=false ;;
  esac
}

if $all; then
  backend=true codegen=true image=true proto=true ts=true ts_pkgs=false
else
  while IFS= read -r f; do
    [[ -n "$f" ]] && classify "$f"
  done
fi

filter=""
if $ts && $ts_pkgs; then filter="--affected"; fi

echo "backend=$backend"
echo "codegen=$codegen"
echo "image=$image"
echo "proto=$proto"
echo "ts=$ts"
echo "ts_filter=$filter"
