#!/usr/bin/env bash
# Decides which CI jobs a change needs. Reads the changed file paths on stdin
# (one per line) and prints `name=value` lines for $GITHUB_OUTPUT:
#   backend, codegen, image, proto, ts   true/false: run that job
#   admin, mobile                        true/false: rebuild that app (admin SPA
#                                        build, mobile web export) in typescript
#   ts_filter                          "--affected" when turbo may test only
#                                        the packages a change reaches, else ""
# `--all` ignores stdin and turns everything on (pushes to main, dispatches).
#
# Fail safe: a path no rule below knows turns everything on. Skipping a check
# by mistake is worse than running one too many. Each group says why its paths
# belong to it; tests are in scripts/ci-changes.test.sh.
#
# Cross-tree reads are the way a rule goes wrong, so they are listed with the
# job that reads them (found by grepping for go:embed and `../` in Go code,
# the Makefile and the scripts): backend/internal/translate/guide.md (embedded),
# data/ and supabase/templates (read by Go tests), supabase/config.toml and
# apps/mobile/src/i18n (read or rewritten by cmd/tool and its tests),
# backend/internal/plans/events.go and crews/notifications.go (read by the
# mobile Jest test push-kinds.test.ts, so a change there runs typescript).
# Re-check when adding a go:embed or a test that reads outside its package.
set -euo pipefail

backend=false codegen=false image=false proto=false ts=false admin=false mobile=false
# ts_pkgs: every TypeScript-relevant path was inside apps/ or packages/, where
# turbo's own dependency graph can pick the packages.
ts_pkgs=true
all=false
[[ "${1:-}" == "--all" ]] && all=true

# Order matters: `*` in a case pattern also matches `/`, so the code arms come
# first and the "no job reads this" arm (which has `*.md`) comes after them.
classify() {
  case "$1" in
    # --- Generated code. The drift check in `codegen` fails when it is edited
    # by hand or goes stale. api-client's package.json pins protoc-gen-es.
    docs/generated/*) codegen=true ;;
    packages/api-client/src/gen/* | packages/api-client/package.json)
      codegen=true ts=true admin=true mobile=true ;;

    # --- Go dependencies. Besides backend, `go tool buf` and the generators
    # are pinned by tool directives here, so a bump can change the proto checks.
    backend/go.mod | backend/go.sum)
      backend=true image=true codegen=true proto=true ts_pkgs=false ;;

    # --- Two Go files the mobile app's Jest suite reads: push-kinds.test.ts
    # parses their Notify* constants to keep apps/mobile's pushKinds table in
    # step. Everything backend/* does, plus the typescript job; turbo's
    # --affected wouldn't pick apps/mobile for a backend-only change.
    backend/internal/plans/events.go | backend/internal/crews/notifications.go)
      backend=true image=true codegen=true ts=true ts_pkgs=false ;;

    # --- Go. The backend job (vet, unit tests, migrations, DB integration
    # tests) and the deploy image both build all of backend/. Markdown under
    # backend/ counts too: translate/guide.md is go:embed'ded. go.work wires
    # the module. codegen too: its generators (cmd/tool ui-strings, sqlc, buf
    # plugins) are compiled from the whole backend module, so any Go file can
    # break `make gen`.
    backend/* | go.work | go.work.sum | .dockerignore)
      backend=true image=true codegen=true ;;

    # data/ is read by Go tests and copied into the image. The app's bundled
    # translations (apps/mobile/src/i18n/bundled) are built from the city
    # packs, so TypeScript runs unfiltered too.
    data/*) backend=true image=true codegen=true ts=true ts_pkgs=false ;;

    # --- Files outside backend/ that Go code reads: emailtemplates_test.go
    # reads supabase/templates/source.json, and `tool emailtemplates` rewrites
    # supabase/config.toml.
    supabase/templates/* | supabase/config.toml) backend=true ;;

    # --- The mobile app's strings. The ui-strings generator reads them (so
    # codegen), and i18n_test.go in the Go module parses i18n/device.ts (so
    # backend).
    apps/mobile/src/i18n/*) ts=true codegen=true backend=true mobile=true ;;

    # --- Protobuf. Generates Go and TS code (codegen drift, api-client) and is
    # what `buf breaking` compares; the Go stubs compile in backend, and the
    # TS client is typechecked in typescript. Turbo gets no filter: the
    # generated client's dependents are cheap to test and easy to miss.
    proto/* | buf.gen.yaml | buf.yaml | buf.work.yaml)
      proto=true codegen=true backend=true image=true ts=true ts_pkgs=false ;;

    # --- Paths no job reads: documentation (also Markdown inside apps/ and
    # packages/), tooling config, deploy and hosting config (checked by the
    # deploy workflows, not by CI), the static site and brand assets.
    docs/* | *.md | .claude/* | site/* | assets/* | config/* | .kamal/* | supabase/* \
      | .githooks/* | .gitignore | .worktreeinclude | .gitleaks.toml | .env.example \
      | version.txt | .release-please-manifest.json | release-please-config.json \
      | skills-lock.json | commitlint.config.mjs | .github/pull_request_template.md \
      | .github/ISSUE_TEMPLATE/* | .github/CODEOWNERS | .github/dependabot.yml) ;;

    # --- TypeScript packages: turbo's graph picks the affected ones.
    # The admin SPA build and the mobile web export (extra steps in that job)
    # follow the app they build; both apps depend on packages/.
    apps/admin/*) ts=true admin=true ;;
    apps/mobile/*) ts=true mobile=true ;;
    apps/* | packages/*) ts=true admin=true mobile=true ;;

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
# Any rule that turned turbo's filter off (toolchain, proto, data, unknown
# paths) rebuilds both apps too.
if $ts && ! $ts_pkgs; then admin=true mobile=true; fi

echo "backend=$backend"
echo "codegen=$codegen"
echo "image=$image"
echo "proto=$proto"
echo "ts=$ts"
echo "ts_filter=$filter"
echo "admin=$admin"
echo "mobile=$mobile"
